package voice

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	"github.com/asomervell/probably/internal/config"
	"google.golang.org/api/option"
	"google.golang.org/genai"
)

// GeminiLiveClient wraps the Gemini Live API for two-way voice conversations
type GeminiLiveClient struct {
	client       *genai.Client
	speechClient *speech.Client
	project      string
	location     string
	model        string
}

// StreamingResult represents a transcription result with its finality status
type StreamingResult struct {
	Transcript string
	IsFinal    bool
}

// StreamingRecognitionSession manages a streaming recognition session
type StreamingRecognitionSession struct {
	stream  speechpb.Speech_StreamingRecognizeClient
	ctx     context.Context
	cancel  context.CancelFunc
	results chan StreamingResult
	errors  chan error
}

// NewGeminiLiveClient creates a new Gemini Live API client
func NewGeminiLiveClient(cfg *config.Config, genaiClient *genai.Client) (*GeminiLiveClient, error) {
	if cfg.VertexProject == "" {
		return nil, fmt.Errorf("VERTEX_PROJECT is required for Gemini Live API")
	}

	model := "gemini-2.0-flash-exp" // Gemini Live compatible model
	if provider, m, ok := strings.Cut(cfg.LLMDefaultModel, "/"); ok && provider == "google" {
		model = m
	}

	// Initialize Speech-to-Text client
	ctx := context.Background()
	speechClient, err := speech.NewClient(ctx, option.WithQuotaProject(cfg.VertexProject))
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "PermissionDenied") || strings.Contains(errStr, "serviceusage") {
			return nil, fmt.Errorf("Speech-to-Text API permission denied - enable API and grant Service Usage Consumer role in GCP console: %w", err)
		}
		return nil, fmt.Errorf("failed to create Speech-to-Text client: %w", err)
	}

	return &GeminiLiveClient{
		client:       genaiClient,
		speechClient: speechClient,
		project:      cfg.VertexProject,
		location:     cfg.VertexLocation,
		model:        model,
	}, nil
}

// ProcessAudio processes incoming audio and returns transcribed text
// audioData should be raw 16-bit PCM audio at 16kHz sample rate
func (c *GeminiLiveClient) ProcessAudio(ctx context.Context, audioData []byte) (string, error) {
	if c.speechClient == nil {
		return "", fmt.Errorf("Speech-to-Text client not initialized")
	}

	// Configure recognition request
	req := &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding:        speechpb.RecognitionConfig_LINEAR16,
			SampleRateHertz: 16000,
			LanguageCode:    "en-US",
			Model:           "latest_long", // Use latest long-form model for better accuracy
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{
				Content: audioData,
			},
		},
	}

	// Perform recognition
	resp, err := c.speechClient.Recognize(ctx, req)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "PermissionDenied") || strings.Contains(errStr, "serviceusage.serviceUsageConsumer") {
			return "", fmt.Errorf("Speech-to-Text API permission denied - enable API and grant Service Usage Consumer role: %w", err)
		}
		return "", fmt.Errorf("Speech-to-Text recognition failed: %w", err)
	}

	// Extract transcription from results
	if len(resp.Results) == 0 {
		return "", fmt.Errorf("no transcription results returned (audio length: %d bytes, ~%.2f seconds)", len(audioData), float64(len(audioData))/32000.0)
	}

	// Combine all alternatives (usually just one, but handle multiple)
	var transcript strings.Builder
	for _, result := range resp.Results {
		if len(result.Alternatives) == 0 {
			continue
		}
		// Use the first (highest confidence) alternative
		transcript.WriteString(result.Alternatives[0].Transcript)
	}

	transcription := strings.TrimSpace(transcript.String())
	if transcription == "" {
		return "", fmt.Errorf("empty transcription result")
	}

	slog.InfoContext(ctx, "audio transcribed", "bytes", len(audioData), "transcription", transcription)
	return transcription, nil
}

// StartStreamingRecognition starts a streaming recognition session for real-time transcription
func (c *GeminiLiveClient) StartStreamingRecognition(ctx context.Context) (*StreamingRecognitionSession, error) {
	if c.speechClient == nil {
		return nil, fmt.Errorf("Speech-to-Text client not initialized")
	}

	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := c.speechClient.StreamingRecognize(streamCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create streaming recognition: %w", err)
	}

	// Send initial configuration
	config := &speechpb.StreamingRecognitionConfig{
		Config: &speechpb.RecognitionConfig{
			Encoding:                   speechpb.RecognitionConfig_LINEAR16,
			SampleRateHertz:            16000,
			LanguageCode:               "en-US",
			Model:                      "latest_long",
			EnableAutomaticPunctuation: true,
		},
		InterimResults:  true,  // Enable interim results for real-time feedback
		SingleUtterance: false, // Keep session open for multiple utterances (we'll manage closing)
	}

	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: config,
		},
	}); err != nil {
		cancel()
		_ = stream.CloseSend()
		return nil, fmt.Errorf("failed to send streaming config: %w", err)
	}

	slog.InfoContext(ctx, "Streaming recognition config sent successfully")

	session := &StreamingRecognitionSession{
		stream:  stream,
		ctx:     streamCtx,
		cancel:  cancel,
		results: make(chan StreamingResult, 10),
		errors:  make(chan error, 1),
	}

	// Start goroutine to receive results
	go func() {
		defer close(session.results)
		defer close(session.errors)

		slog.InfoContext(ctx, "Starting to receive streaming recognition results...")

		for {
			resp, err := stream.Recv()
			if err != nil {
				if err != io.EOF {
					slog.ErrorContext(ctx, "Streaming recognition receive error", "err", err)
					session.errors <- err
				} else {
					slog.InfoContext(ctx, "Streaming recognition stream ended (EOF)")
				}
				return
			}

			slog.DebugContext(ctx, "received streaming response", "results", len(resp.Results))

			// Process results
			for i, result := range resp.Results {
				slog.DebugContext(ctx, "streaming result", "index", i, "is_final", result.IsFinal, "alternatives", len(result.Alternatives))

				if len(result.Alternatives) == 0 {
					slog.DebugContext(ctx, "streaming result has no alternatives", "index", i)
					continue
				}

				transcript := result.Alternatives[0].Transcript
				if transcript != "" {
					slog.DebugContext(ctx, "sending transcription result", "is_final", result.IsFinal, "transcript", transcript)
					session.results <- StreamingResult{
						Transcript: transcript,
						IsFinal:    result.IsFinal,
					}
				} else {
					slog.DebugContext(ctx, "streaming result has empty transcript", "index", i)
				}
			}
		}
	}()

	return session, nil
}

// SendAudio sends audio data to the streaming recognition session
func (s *StreamingRecognitionSession) SendAudio(audioData []byte) error {
	return s.stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
			AudioContent: audioData,
		},
	})
}

// Close stops the streaming recognition session
func (s *StreamingRecognitionSession) Close() error {
	s.cancel()
	return s.stream.CloseSend()
}

// GetResults returns the channel for receiving transcription results
func (s *StreamingRecognitionSession) GetResults() <-chan StreamingResult {
	return s.results
}

// GetErrors returns the channel for receiving errors
func (s *StreamingRecognitionSession) GetErrors() <-chan error {
	return s.errors
}

// Close closes the client and cleans up resources
func (c *GeminiLiveClient) Close() error {
	if c.speechClient != nil {
		return c.speechClient.Close()
	}
	// genai client is shared, don't close it here
	return nil
}
