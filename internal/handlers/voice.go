package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/chat"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/orchestrator"
	"github.com/asomervell/probably/internal/voice"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"sync"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10
)

// voiceUpgrader returns a WebSocket upgrader that validates the Origin header
// against the configured base URL, preventing cross-site WebSocket hijacking.
func (hdl *Handlers) voiceUpgrader() *websocket.Upgrader {
	allowed := hdl.cfg.BaseURL
	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // same-origin requests carry no Origin header
			}
			return origin == allowed
		},
	}
}

// VoiceMessage represents a message in the voice conversation
type VoiceMessage struct {
	Type      string `json:"type"`      // "audio", "text", "transcription", "error"
	Data      string `json:"data"`      // Audio data (base64 encoded string) or text
	Text      string `json:"text"`      // Text content (for transcription/TTS)
	Timestamp int64  `json:"timestamp"` // Unix timestamp in milliseconds
}

// VoiceConnection represents a WebSocket connection for voice chat
type VoiceConnection struct {
	conn               *websocket.Conn
	ledgerID           uuid.UUID
	userID             uuid.UUID
	threadID           *uuid.UUID   // Thread ID for this conversation (can be nil for new thread)
	thread             *chat.Thread // Thread object (loaded or created)
	voiceSvc           *voice.GeminiLiveClient
	orch               *orchestrator.Orchestrator
	hdl                *Handlers // Reference to handlers for config access
	send               chan VoiceMessage
	done               chan bool
	audioBuffer        []byte                             // Buffer for accumulating raw PCM audio bytes (decoded from base64 chunks)
	streamingSession   *voice.StreamingRecognitionSession // Streaming recognition session for real-time transcription
	streamingMutex     sync.Mutex                         // Protect streaming session access
	finalTranscription string                             // Store final transcription when speech ends
	processingSpeech   bool                               // Flag to track if we're currently processing end of speech (stop sending audio)
}

// VoiceChat handles WebSocket connections for voice conversations
func (hdl *Handlers) VoiceChat(w http.ResponseWriter, r *http.Request) {
	if !hdl.cfg.VoiceEnabled {
		http.Error(w, "Voice chat is not enabled", http.StatusServiceUnavailable)
		return
	}

	// Get user and ledger
	user := auth.APICurrentUser(r)
	if user == nil {
		user = auth.CurrentUser(r)
	}
	if user == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	ledgerIDStr := r.URL.Query().Get("ledger_id")
	if ledgerIDStr == "" {
		http.Error(w, "ledger_id is required", http.StatusBadRequest)
		return
	}

	ledgerID, err := uuid.Parse(ledgerIDStr)
	if err != nil {
		http.Error(w, "invalid ledger_id", http.StatusBadRequest)
		return
	}

	// Verify ledger access
	_, err = hdl.permissionChecker.CheckLedgerPermission(r.Context(), user.ID, ledgerID, models.PermissionLevelView)
	if err != nil {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	// Get or create thread (same as text chat)
	threadIDStr := r.URL.Query().Get("thread_id")
	var thread *chat.Thread
	var threadUUID *uuid.UUID

	ctx := r.Context()
	if threadIDStr != "" {
		// Load existing thread
		tid, err := uuid.Parse(threadIDStr)
		if err == nil {
			thread, err = hdl.chatThreads.GetThreadForUser(ctx, tid, ledgerID, user.ID)
			if err == nil {
				threadUUID = &tid
			} else {
				slog.ErrorContext(r.Context(), "Failed to load thread", "id", threadIDStr, "err", err)
				// Continue without thread - will create new one on first message
			}
		}
	}

	// If no thread loaded, we'll create one on first message

	// Upgrade to WebSocket
	conn, err := hdl.voiceUpgrader().Upgrade(w, r, nil)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to upgrade connection", "err", err)
		return
	}

	// Create orchestrator
	orch, err := orchestrator.NewOrchestrator(hdl.cfg)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to create orchestrator", "err", err)
		conn.Close()
		return
	}

	// Get Vertex AI client from orchestrator (if available) and create voice client
	var voiceClient *voice.GeminiLiveClient
	if orch != nil {
		genaiClient := orch.GetVertexClient()
		if genaiClient != nil {
			client, err := voice.NewGeminiLiveClient(hdl.cfg, genaiClient)
			if err != nil {
				slog.ErrorContext(r.Context(), "Failed to create voice client", "err", err)
				// Continue without voice client - will use fallback
			} else {
				voiceClient = client
				slog.InfoContext(r.Context(), "Voice client initialized successfully")
			}
		} else {
			slog.InfoContext(r.Context(), "Vertex AI client not available - voice transcription disabled")
		}
	}

	// Create connection handler
	vc := &VoiceConnection{
		conn:     conn,
		ledgerID: ledgerID,
		userID:   user.ID,
		threadID: threadUUID,
		thread:   thread,
		voiceSvc: voiceClient,
		orch:     orch,
		hdl:      hdl,
		send:     make(chan VoiceMessage, 256),
		done:     make(chan bool),
	}

	// Send thread ID to client if we have one
	if threadUUID != nil {
		response := VoiceMessage{
			Type:      "thread_id",
			Text:      threadUUID.String(),
			Timestamp: time.Now().UnixMilli(),
		}
		vc.send <- response
	}

	// Start connection handlers
	go vc.writePump()
	go vc.readPump()

	// Start streaming recognition session for real-time transcription
	if voiceClient != nil {
		ctx := context.Background()
		session, err := voiceClient.StartStreamingRecognition(ctx)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to start streaming recognition, will use batch mode", "err", err)
			// Check if it's a permission/API not enabled error
			if strings.Contains(err.Error(), "PermissionDenied") || strings.Contains(err.Error(), "not been used") || strings.Contains(err.Error(), "disabled") {
				slog.InfoContext(r.Context(), "Speech-to-Text API may not be enabled", "project", hdl.cfg.VertexProject)
			}
		} else {
			slog.InfoContext(r.Context(), "Streaming recognition session started successfully")
			vc.streamingMutex.Lock()
			vc.streamingSession = session
			vc.streamingMutex.Unlock()

			// Start goroutine to handle streaming results
			go vc.handleStreamingResults(session)
		}
	}
}

// handleStreamingResults processes real-time transcription results from streaming API
func (vc *VoiceConnection) handleStreamingResults(session *voice.StreamingRecognitionSession) {
	defer func() {
		vc.streamingMutex.Lock()
		vc.streamingSession = nil
		vc.streamingMutex.Unlock()
	}()

	for {
		select {
		case result, ok := <-session.GetResults():
			if !ok {
				return
			}

			if result.IsFinal {
				// Store final transcription
				vc.streamingMutex.Lock()
				vc.finalTranscription = result.Transcript
				vc.streamingMutex.Unlock()
				slog.Debug("streaming transcription (final)", "transcript", result.Transcript)
			} else {
				// Send partial transcription to client for real-time display
				transcriptionMsg := VoiceMessage{
					Type:      "transcription",
					Text:      result.Transcript,
					Timestamp: time.Now().UnixMilli(),
				}
				vc.send <- transcriptionMsg
				slog.Debug("streaming transcription (interim)", "transcript", result.Transcript)
			}

		case err, ok := <-session.GetErrors():
			if !ok {
				return
			}
			slog.Error("Streaming recognition error", "err", err)
			vc.sendError(fmt.Sprintf("Transcription error: %v", err))
			return

		case <-vc.done:
			return
		}
	}
}

// readPump pumps messages from the WebSocket connection
func (vc *VoiceConnection) readPump() {
	defer func() {
		// Clean up audio buffer for this connection
		vc.audioBuffer = nil
		// Close voice client if it was initialized
		if vc.voiceSvc != nil {
			if err := vc.voiceSvc.Close(); err != nil {
				slog.Error("Error closing voice client", "err", err)
			}
		}
		vc.conn.Close()
		close(vc.done)
	}()

	_ = vc.conn.SetReadDeadline(time.Now().Add(pongWait))
	vc.conn.SetPongHandler(func(string) error {
		_ = vc.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := vc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket error", "err", err)
			}
			break
		}

		var msg VoiceMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			slog.Error("Failed to unmarshal message", "err", err)
			continue
		}

		vc.handleMessage(&msg)
	}
}

// writePump pumps messages to the WebSocket connection
func (vc *VoiceConnection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		vc.conn.Close()
	}()

	for {
		select {
		case message, ok := <-vc.send:
			_ = vc.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = vc.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := vc.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			jsonData, err := json.Marshal(message)
			if err != nil {
				slog.Error("Failed to marshal message", "err", err)
				w.Close()
				continue
			}

			_, _ = w.Write(jsonData)

			// Close writer to flush message
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = vc.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := vc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-vc.done:
			return
		}
	}
}

// handleMessage processes incoming messages from the client
func (vc *VoiceConnection) handleMessage(msg *VoiceMessage) {
	switch msg.Type {
	case "audio":
		// Handle incoming audio chunk
		vc.handleAudioChunk(msg.Data)

	case "end_speech":
		// Client detected end of speech - process buffered audio
		vc.handleEndOfSpeech()

	case "text":
		// Handle text input (fallback or manual input)
		vc.handleTextInput(msg.Text)

	case "transcription":
		// Client has transcribed audio, process as text
		vc.handleTextInput(msg.Text)

	default:
		slog.Warn("unknown message type", "type", msg.Type)
	}
}

// Note: Audio buffers are stored per-connection in VoiceConnection struct

// handleAudioChunk processes an incoming audio chunk
// audioData is a base64-encoded string containing 16-bit PCM audio at 16kHz
func (vc *VoiceConnection) handleAudioChunk(audioData string) {
	// Decode base64 chunk immediately and append raw PCM audio bytes
	// This avoids issues with concatenating base64 strings (padding breaks format)
	decoded, err := base64.StdEncoding.DecodeString(audioData)
	if err != nil {
		slog.Error("failed to decode base64 audio chunk", "err", err, "data_len", len(audioData))
		// Send error but don't break the connection - continue with next chunk
		vc.sendError("Failed to decode audio chunk")
		return
	}

	// Check if we're currently processing end of speech
	// Once end_speech is received, we stop accepting new audio chunks from frontend
	// The backend will handle the grace period by sending its own silence chunks
	vc.streamingMutex.Lock()
	processingSpeech := vc.processingSpeech
	vc.streamingMutex.Unlock()

	// If we're processing speech (end_speech received), reject new audio chunks
	// The backend handles the grace period internally with silence chunks
	if processingSpeech {
		slog.Debug("Rejecting audio chunk - end_speech already received, processing transcription")
		// Still send ack to frontend so it knows we received it
		response := VoiceMessage{
			Type:      "ack",
			Timestamp: time.Now().UnixMilli(),
		}
		vc.send <- response
		return
	}

	// Append decoded raw PCM audio bytes to buffer (for fallback batch processing)
	vc.audioBuffer = append(vc.audioBuffer, decoded...)

	// Send to streaming recognition if session is available
	// If no session exists (e.g., previous one closed), start a new one
	vc.streamingMutex.Lock()
	session := vc.streamingSession
	vc.streamingMutex.Unlock()

	if session == nil && vc.voiceSvc != nil {
		slog.Debug("Starting new streaming recognition session for next utterance")
		ctx := context.Background()
		newSession, err := vc.voiceSvc.StartStreamingRecognition(ctx)
		if err != nil {
			slog.Error("Failed to start new streaming recognition", "err", err)
		} else {
			vc.streamingMutex.Lock()
			vc.streamingSession = newSession
			session = newSession
			vc.streamingMutex.Unlock()

			// Start goroutine to handle streaming results for this new session
			go vc.handleStreamingResults(newSession)
		}
	}

	if session != nil {
		if err := session.SendAudio(decoded); err != nil {
			slog.Error("Failed to send audio to streaming recognition", "err", err)
			// Mark session as failed so we don't try to use it
			vc.streamingMutex.Lock()
			vc.streamingSession = nil
			vc.streamingMutex.Unlock()
		} else {
			slog.Debug("sent bytes to streaming recognition", "bytes", len(decoded))
		}
	}

	// Send acknowledgment
	response := VoiceMessage{
		Type:      "ack",
		Timestamp: time.Now().UnixMilli(),
	}
	vc.send <- response

	slog.Debug("voice: buffered audio chunk", "base64_bytes", len(audioData), "raw_bytes", len(decoded), "total_buffered", len(vc.audioBuffer))
}

// handleEndOfSpeech processes buffered audio when user stops speaking
func (vc *VoiceConnection) handleEndOfSpeech() {
	if len(vc.audioBuffer) == 0 {
		slog.Debug("No audio buffered for end-of-speech")
		vc.sendError("No audio received")
		return
	}

	// Set flag to stop sending audio chunks to streaming session
	vc.streamingMutex.Lock()
	vc.processingSpeech = true
	session := vc.streamingSession
	vc.streamingMutex.Unlock()

	// Close the streaming session immediately to tell the API we're done
	// This triggers the API to process remaining audio and return the final result
	if session != nil {
		slog.Debug("Closing streaming session to finalize transcription")
		if err := session.Close(); err != nil {
			slog.Error("Error closing streaming session", "err", err)
		}

		// Mark session as nil so we don't try to use it again
		vc.streamingMutex.Lock()
		vc.streamingSession = nil
		vc.streamingMutex.Unlock()
	}

	// Send processing status
	response := VoiceMessage{
		Type:      "processing",
		Timestamp: time.Now().UnixMilli(),
		Text:      "Processing your question...",
	}
	vc.send <- response

	slog.Debug("end-of-speech detected", "buffered_bytes", len(vc.audioBuffer))

	var transcription string
	var err error

	if session != nil {
		// The streaming API should automatically send final results when it detects end of speech
		// Wait for final transcription (with timeout)
		maxWait := 3 * time.Second
		checkInterval := 200 * time.Millisecond
		waited := time.Duration(0)

		slog.Debug("waiting for final transcription from streaming API", "max_wait", maxWait)

		for waited < maxWait {
			vc.streamingMutex.Lock()
			transcription = vc.finalTranscription
			vc.streamingMutex.Unlock()

			if transcription != "" {
				slog.Debug("got final transcription from streaming", "transcription", transcription, "waited", waited)
				// Don't close session - keep it open for next utterance
				break
			}

			time.Sleep(checkInterval)
			waited += checkInterval
		}

		// If we didn't get a final result, the streaming API might not be working
		// Fall back to batch processing (but keep session open in case it starts working)
		if transcription == "" {
			slog.Debug("no final transcription from streaming, falling back to batch", "waited", waited)
			// Don't close session - it might start working, and we want to keep it for next utterance
			// Fall back to batch processing
			transcription, err = vc.fallbackBatchTranscription()
		}
	} else {
		// No streaming session, use batch processing
		slog.Debug("No streaming session available, using batch transcription")
		transcription, err = vc.fallbackBatchTranscription()
	}

	if err != nil {
		slog.Error("Speech-to-Text failed", "err", err)
		vc.sendError(fmt.Sprintf("Failed to transcribe audio: %v", err))
		return
	}

	if transcription == "" {
		slog.Debug("Empty transcription result")
		vc.sendError("Could not understand audio. Please try again.")
		return
	}

	slog.Debug("final transcription", "transcript", transcription)

	// Send final transcription to client (may have already been sent via streaming, but send again for clarity)
	transcriptionMsg := VoiceMessage{
		Type:      "transcription",
		Text:      transcription,
		Timestamp: time.Now().UnixMilli(),
	}
	vc.send <- transcriptionMsg

	// Clear buffer for next utterance and reset processing flag
	vc.audioBuffer = nil
	vc.streamingMutex.Lock()
	vc.finalTranscription = ""
	vc.processingSpeech = false // Reset flag to allow new audio chunks for next utterance
	// Keep streaming session closed - it will be restarted in handleAudioChunk
	vc.streamingMutex.Unlock()

	// Process the transcribed text as if it were typed
	vc.handleTextInput(transcription)
}

// fallbackBatchTranscription uses batch API as fallback when streaming is not available
func (vc *VoiceConnection) fallbackBatchTranscription() (string, error) {
	if vc.voiceSvc == nil {
		return "", fmt.Errorf("voice service not initialized")
	}

	audioData := vc.audioBuffer
	if len(audioData) == 0 {
		return "", fmt.Errorf("no audio data to transcribe")
	}

	// Ensure minimum audio length (at least 0.1 seconds at 16kHz = 3200 bytes)
	minAudioBytes := 3200
	if len(audioData) < minAudioBytes {
		return "", fmt.Errorf("audio too short (%d bytes, need at least %d bytes)", len(audioData), minAudioBytes)
	}

	ctx := context.Background()
	return vc.voiceSvc.ProcessAudio(ctx, audioData)
}

// handleTextInput processes text input (from transcription or manual entry)
func (vc *VoiceConnection) handleTextInput(text string) {
	if text == "" {
		return
	}

	slog.Debug("processing text input", "text", text)

	// Ensure we have a thread (create if needed)
	ctx := context.Background()
	if vc.thread == nil {
		thread, err := vc.hdl.chatThreads.CreateThread(ctx, vc.ledgerID, vc.userID, nil)
		if err != nil {
			slog.Error("Failed to create thread", "err", err)
			vc.sendError("Failed to create conversation thread")
			return
		}
		vc.thread = thread
		vc.threadID = &thread.ID

		// Notify client of new thread ID
		response := VoiceMessage{
			Type:      "thread_id",
			Text:      thread.ID.String(),
			Timestamp: time.Now().UnixMilli(),
		}
		vc.send <- response
	}

	// Build conversation history from thread messages
	var history string
	messages, err := vc.hdl.chatThreads.GetMessages(ctx, vc.thread.ID)
	if err == nil && len(messages) > 0 {
		history = buildHistoryFromMessages(messages)
	}

	// Check for similarity queries and enhance with embedding search results
	var similarityContext string
	if vc.hdl.embeddingService != nil && vc.hdl.embeddingService.IsConfigured() {
		if ctx, found := vc.hdl.detectAndFetchSimilarTransactions(ctx, text, vc.ledgerID); found {
			similarityContext = ctx
		}
	}

	// Build prompt (with optional similarity context)
	systemPrompt := chat.BuildChatSystemPromptVoice()
	userPrompt := chat.BuildChatUserPrompt(text, history)
	if similarityContext != "" {
		userPrompt = userPrompt + "\n\n" + similarityContext
	}

	// Create chat task with voice-optimized prompt
	task := &orchestrator.Task{
		Type:     orchestrator.TaskTypeChatSQL,
		Strategy: orchestrator.Strategy(vc.hdl.cfg.LLMChatStrategy),
		Input: &orchestrator.ChatInput{
			Messages: []interface{}{
				map[string]interface{}{"role": "system", "content": systemPrompt},
				map[string]interface{}{"role": "user", "content": userPrompt},
			},
			GenerateSQL:     true,
			VoiceMode:       true, // Enable voice-optimized responses
			ThoughtCallback: nil,  // No streaming thoughts for voice
		},
		Context: &orchestrator.TaskContext{
			LedgerID: vc.ledgerID.String(),
			UserID:   vc.userID.String(),
		},
	}

	// Execute task
	result, err := vc.orch.Execute(ctx, task)
	if err != nil {
		slog.Error("Task execution failed", "err", err)
		vc.sendError(fmt.Sprintf("Failed to process request: %v", err))
		return
	}

	// Extract response - result.Output is a map[string]interface{} for chat tasks
	chatResp, ok := result.Output.(map[string]interface{})
	if !ok {
		vc.sendError("Invalid response format")
		return
	}

	// Extract SQL from response (internal only, never sent to client)
	sql, _ := chatResp["sql"].(string)
	answerTemplate, _ := chatResp["answer_template"].(string)

	if sql == "" {
		vc.sendError("Failed to generate valid query")
		return
	}

	// Validate SQL (server-side only)
	validator := chat.NewSQLValidator()
	if err := validator.Validate(sql, vc.ledgerID.String()); err != nil {
		slog.Error("SQL validation failed", "err", err)
		vc.sendError("Generated query failed validation")
		return
	}

	// Execute SQL (server-side only)
	executor := chat.NewSQLExecutor(vc.hdl.db.Pool)
	queryResult, err := executor.Execute(ctx, sql, vc.ledgerID, 10000)
	if err != nil {
		slog.Error("SQL execution failed", "err", err)
		vc.sendError("Query execution failed")
		return
	}

	// Format answer using template (this is markdown)
	answer := chat.FormatAnswer(answerTemplate, queryResult)

	// Convert markdown to HTML for frontend rendering (same as regular chat)
	// Note: renderMarkdownToHTML is in chat.go (same package)
	answerHTML := renderMarkdownToHTML(answer)

	// Store conversation messages in context (store original markdown, not HTML)
	convCtx := vc.hdl.chatContext.GetOrCreate(vc.userID, vc.ledgerID)
	vc.hdl.chatContext.AddMessage(convCtx, "user", text, "", nil)
	vc.hdl.chatContext.AddMessage(convCtx, "assistant", answer, sql, queryResult)

	// Persist messages to database (store original markdown, not HTML)
	_, err = vc.hdl.chatThreads.AddMessage(ctx, vc.thread.ID, "user", text, "", nil)
	if err != nil {
		slog.Error("Failed to save user message", "err", err)
	}
	_, err = vc.hdl.chatThreads.AddMessage(ctx, vc.thread.ID, "assistant", answer, sql, queryResult)
	if err != nil {
		slog.Error("Failed to save assistant message", "err", err)
	}

	// Generate title for new threads (async)
	if vc.threadID != nil && vc.thread.Title == "" {
		go vc.hdl.generateThreadTitle(vc.thread.ID, text)
	}

	// Send HTML response (pre-rendered markdown, same format as regular chat)
	vc.sendTextResponse(answerHTML)

	// TODO: Generate TTS audio and stream it
	// For now, just send the text
}

// sendTextResponse sends a text response to the client
func (vc *VoiceConnection) sendTextResponse(text string) {
	response := VoiceMessage{
		Type:      "text",
		Text:      text,
		Timestamp: time.Now().UnixMilli(),
	}
	vc.send <- response
}

// sendError sends an error message to the client
func (vc *VoiceConnection) sendError(errMsg string) {
	response := VoiceMessage{
		Type:      "error",
		Text:      errMsg,
		Timestamp: time.Now().UnixMilli(),
	}
	vc.send <- response
}
