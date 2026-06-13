package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
)

// WebAuthnUser implements webauthn.User interface
type WebAuthnUser struct {
	user     *models.User
	passkeys []*models.Passkey
}

// NewWebAuthnUser creates a new WebAuthnUser
func NewWebAuthnUser(user *models.User, passkeys []*models.Passkey) *WebAuthnUser {
	return &WebAuthnUser{
		user:     user,
		passkeys: passkeys,
	}
}

// WebAuthnID returns the user's ID as bytes (required by webauthn.User)
func (u *WebAuthnUser) WebAuthnID() []byte {
	return u.user.ID[:]
}

// WebAuthnName returns the user's email (required by webauthn.User)
func (u *WebAuthnUser) WebAuthnName() string {
	return u.user.Email
}

// WebAuthnDisplayName returns the user's display name (required by webauthn.User)
func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.user.Email
}

// WebAuthnCredentials returns the user's credentials (required by webauthn.User)
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	credentials := make([]webauthn.Credential, len(u.passkeys))
	for i, p := range u.passkeys {
		credentials[i] = p.ToWebAuthnCredential()
	}
	return credentials
}

// WebAuthnIcon returns an empty string (deprecated field)
func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}

// WebAuthnService handles WebAuthn operations
type WebAuthnService struct {
	webAuthn     *webauthn.WebAuthn
	sessionStore *sessions.CookieStore
	cfg          *config.Config
}

// NewWebAuthnService creates a new WebAuthn service
func NewWebAuthnService(cfg *config.Config, sessionStore *sessions.CookieStore) (*WebAuthnService, error) {
	// Extract domain from BaseURL
	parsedURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse BaseURL: %w", err)
	}

	rpID := parsedURL.Hostname()
	rpOrigins := []string{cfg.BaseURL}

	// For localhost, also allow different ports
	if strings.HasPrefix(rpID, "localhost") || rpID == "127.0.0.1" {
		rpID = "localhost"
		rpOrigins = []string{
			"http://localhost:8080",
			"http://localhost:3000",
			"http://127.0.0.1:8080",
			cfg.BaseURL,
		}
	}

	wconfig := &webauthn.Config{
		RPDisplayName: "Probably",
		RPID:          rpID,
		RPOrigins:     rpOrigins,
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    time.Minute * 5,
				TimeoutUVD: time.Minute * 5,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    time.Minute * 5,
				TimeoutUVD: time.Minute * 5,
			},
		},
	}

	webAuthn, err := webauthn.New(wconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create webauthn: %w", err)
	}

	return &WebAuthnService{
		webAuthn:     webAuthn,
		sessionStore: sessionStore,
		cfg:          cfg,
	}, nil
}

// WebAuthnSessionData stores session data for WebAuthn ceremonies
type WebAuthnSessionData struct {
	Challenge            string    `json:"challenge"`
	UserID               []byte    `json:"user_id"`
	AllowedCredentialIDs [][]byte  `json:"allowed_credential_ids,omitempty"`
	UserVerification     string    `json:"user_verification"`
	ExpiresAt            time.Time `json:"expires_at"`
}

// BeginRegistration starts the WebAuthn registration ceremony
func (s *WebAuthnService) BeginRegistration(w http.ResponseWriter, r *http.Request, user *models.User, passkeys []*models.Passkey) (*protocol.CredentialCreation, error) {
	webAuthnUser := NewWebAuthnUser(user, passkeys)

	// Exclude existing credentials
	excludeList := make([]protocol.CredentialDescriptor, len(passkeys))
	for i, p := range passkeys {
		excludeList[i] = protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: p.CredentialID,
		}
	}

	options, sessionData, err := s.webAuthn.BeginRegistration(
		webAuthnUser,
		webauthn.WithExclusions(excludeList),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			AuthenticatorAttachment: protocol.CrossPlatform,
			UserVerification:        protocol.VerificationPreferred,
			ResidentKey:             protocol.ResidentKeyRequirementPreferred,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to begin registration: %w", err)
	}

	// Store session data
	if err := s.storeSessionData(w, r, "webauthn_register", sessionData); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return options, nil
}

// FinishRegistration completes the WebAuthn registration ceremony
func (s *WebAuthnService) FinishRegistration(w http.ResponseWriter, r *http.Request, user *models.User, passkeys []*models.Passkey) (*webauthn.Credential, error) {
	webAuthnUser := NewWebAuthnUser(user, passkeys)

	// Get session data
	sessionData, err := s.getSessionData(r, "webauthn_register")
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	credential, err := s.webAuthn.FinishRegistration(webAuthnUser, *sessionData, r)
	if err != nil {
		return nil, fmt.Errorf("failed to finish registration: %w", err)
	}

	// Clear session data
	s.clearSessionData(w, r, "webauthn_register")

	return credential, nil
}

// BeginLogin starts the WebAuthn login ceremony
func (s *WebAuthnService) BeginLogin(w http.ResponseWriter, r *http.Request, user *models.User, passkeys []*models.Passkey) (*protocol.CredentialAssertion, error) {
	webAuthnUser := NewWebAuthnUser(user, passkeys)

	options, sessionData, err := s.webAuthn.BeginLogin(webAuthnUser)
	if err != nil {
		return nil, fmt.Errorf("failed to begin login: %w", err)
	}

	// Store session data with user ID for later retrieval
	if err := s.storeSessionDataWithUser(w, r, "webauthn_login", sessionData, user.ID); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return options, nil
}

// BeginDiscoverableLogin starts a discoverable (usernameless) WebAuthn login
func (s *WebAuthnService) BeginDiscoverableLogin(w http.ResponseWriter, r *http.Request) (*protocol.CredentialAssertion, error) {
	options, sessionData, err := s.webAuthn.BeginDiscoverableLogin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin discoverable login: %w", err)
	}

	// Store session data without user ID (will be discovered from credential)
	if err := s.storeSessionData(w, r, "webauthn_login", sessionData); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	return options, nil
}

// FinishLogin completes the WebAuthn login ceremony
func (s *WebAuthnService) FinishLogin(w http.ResponseWriter, r *http.Request, user *models.User, passkeys []*models.Passkey) (*webauthn.Credential, error) {
	webAuthnUser := NewWebAuthnUser(user, passkeys)

	// Get session data
	sessionData, err := s.getSessionData(r, "webauthn_login")
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	credential, err := s.webAuthn.FinishLogin(webAuthnUser, *sessionData, r)
	if err != nil {
		return nil, fmt.Errorf("failed to finish login: %w", err)
	}

	// Clear session data
	s.clearSessionData(w, r, "webauthn_login")

	return credential, nil
}

// FinishDiscoverableLogin completes a discoverable WebAuthn login
func (s *WebAuthnService) FinishDiscoverableLogin(w http.ResponseWriter, r *http.Request, userHandler func(rawID, userHandle []byte) (webauthn.User, error)) (*webauthn.Credential, error) {
	// Get session data
	sessionData, err := s.getSessionData(r, "webauthn_login")
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	credential, err := s.webAuthn.FinishDiscoverableLogin(userHandler, *sessionData, r)
	if err != nil {
		return nil, fmt.Errorf("failed to finish discoverable login: %w", err)
	}

	// Clear session data
	s.clearSessionData(w, r, "webauthn_login")

	return credential, nil
}

// storeSessionData stores WebAuthn session data in a cookie
func (s *WebAuthnService) storeSessionData(w http.ResponseWriter, r *http.Request, key string, sessionData *webauthn.SessionData) error {
	session, err := s.sessionStore.Get(r, "webauthn_session")
	if err != nil {
		slog.WarnContext(r.Context(), "session cookie invalid, using fresh session", "err", err)
	}

	// Serialize session data
	data, err := json.Marshal(sessionData)
	if err != nil {
		return err
	}

	session.Values[key] = base64.StdEncoding.EncodeToString(data)
	return session.Save(r, w)
}

// storeSessionDataWithUser stores WebAuthn session data with associated user ID
func (s *WebAuthnService) storeSessionDataWithUser(w http.ResponseWriter, r *http.Request, key string, sessionData *webauthn.SessionData, userID uuid.UUID) error {
	session, err := s.sessionStore.Get(r, "webauthn_session")
	if err != nil {
		slog.WarnContext(r.Context(), "session cookie invalid, using fresh session", "err", err)
	}

	// Serialize session data
	data, err := json.Marshal(sessionData)
	if err != nil {
		return err
	}

	session.Values[key] = base64.StdEncoding.EncodeToString(data)
	session.Values[key+"_user_id"] = userID.String()
	return session.Save(r, w)
}

// getSessionData retrieves WebAuthn session data from a cookie
func (s *WebAuthnService) getSessionData(r *http.Request, key string) (*webauthn.SessionData, error) {
	session, err := s.sessionStore.Get(r, "webauthn_session")
	if err != nil {
		return nil, fmt.Errorf("session not found")
	}

	dataStr, ok := session.Values[key].(string)
	if !ok {
		return nil, fmt.Errorf("session data not found")
	}

	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, err
	}

	var sessionData webauthn.SessionData
	if err := json.Unmarshal(data, &sessionData); err != nil {
		return nil, err
	}

	return &sessionData, nil
}

// GetSessionUserID retrieves the user ID from WebAuthn session
func (s *WebAuthnService) GetSessionUserID(r *http.Request, key string) (uuid.UUID, error) {
	session, err := s.sessionStore.Get(r, "webauthn_session")
	if err != nil {
		return uuid.Nil, fmt.Errorf("session not found")
	}

	userIDStr, ok := session.Values[key+"_user_id"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("user ID not found in session")
	}

	return uuid.Parse(userIDStr)
}

// clearSessionData removes WebAuthn session data
func (s *WebAuthnService) clearSessionData(w http.ResponseWriter, r *http.Request, key string) {
	session, err := s.sessionStore.Get(r, "webauthn_session")
	if err != nil {
		return
	}

	delete(session.Values, key)
	delete(session.Values, key+"_user_id")
	_ = session.Save(r, w)
}

// CreatePasskeyFromCredential creates a Passkey model from a WebAuthn credential
func CreatePasskeyFromCredential(userID uuid.UUID, credential *webauthn.Credential, name string) *models.Passkey {
	transports := make([]string, len(credential.Transport))
	for i, t := range credential.Transport {
		transports[i] = string(t)
	}

	return &models.Passkey{
		UserID:          userID,
		CredentialID:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		Transport:       transports,
		AAGUID:          credential.Authenticator.AAGUID,
		SignCount:       credential.Authenticator.SignCount,
		Name:            name,
	}
}
