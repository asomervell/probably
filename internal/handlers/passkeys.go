package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	g "maragu.dev/gomponents"
	hx "maragu.dev/gomponents/html"
)

// BeginPasskeyRegistration starts the passkey registration ceremony
func (hdl *Handlers) BeginPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get existing passkeys for exclusion
	passkeys, err := hdl.passkeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get passkeys", "err", err)
		passkeys = []*models.Passkey{}
	}

	// Begin registration
	options, err := hdl.webauthn.BeginRegistration(w, r, user, passkeys)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to begin registration", "err", err)
		http.Error(w, "Failed to start registration", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, options)
}

// FinishPasskeyRegistration completes the passkey registration ceremony
func (hdl *Handlers) FinishPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get passkey name from query or default
	name := r.URL.Query().Get("name")
	if name == "" {
		name = fmt.Sprintf("Passkey %s", time.Now().Format("Jan 2, 2006"))
	}

	// Get existing passkeys
	passkeys, err := hdl.passkeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		passkeys = []*models.Passkey{}
	}

	// Finish registration
	credential, err := hdl.webauthn.FinishRegistration(w, r, user, passkeys)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to finish registration", "err", err)
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to complete registration"})
		return
	}

	// Create passkey record
	passkey := auth.CreatePasskeyFromCredential(user.ID, credential, name)
	if err := hdl.passkeys.Create(r.Context(), passkey); err != nil {
		slog.ErrorContext(r.Context(), "Failed to save passkey", "err", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save passkey"})
		return
	}

	slog.InfoContext(r.Context(), "registered new passkey for user", "email", user.Email, "name", name)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      passkey.ID,
		"name":    passkey.Name,
	})
}

// BeginPasskeyLogin starts the passkey login ceremony (discoverable)
func (hdl *Handlers) BeginPasskeyLogin(w http.ResponseWriter, r *http.Request) {
	// Check if email is provided for non-discoverable login
	email := r.URL.Query().Get("email")
	if email == "" {
		// Try to parse from body
		var body struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			email = body.Email
		}
	}

	var options interface{}
	var err error

	if email != "" {
		// Non-discoverable login - user provides email first
		user, userErr := hdl.users.GetByEmail(r.Context(), email)
		if userErr != nil {
			// Don't reveal if user exists
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "No passkeys found"})
			return
		}

		passkeys, _ := hdl.passkeys.GetByUserID(r.Context(), user.ID)
		if len(passkeys) == 0 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "No passkeys found"})
			return
		}

		options, err = hdl.webauthn.BeginLogin(w, r, user, passkeys)
	} else {
		// Discoverable login - let the authenticator select the credential
		options, err = hdl.webauthn.BeginDiscoverableLogin(w, r)
	}

	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to begin login", "err", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to start login"})
		return
	}

	respondJSON(w, http.StatusOK, options)
}

// FinishPasskeyLogin completes the passkey login ceremony
func (hdl *Handlers) FinishPasskeyLogin(w http.ResponseWriter, r *http.Request) {
	// Try discoverable login first (user handle in response will identify user)
	credential, err := hdl.webauthn.FinishDiscoverableLogin(w, r, func(rawID, userHandle []byte) (webauthn.User, error) {
		// userHandle is the user ID we stored during registration
		var userID uuid.UUID
		if len(userHandle) == 16 {
			copy(userID[:], userHandle)
		} else {
			return nil, fmt.Errorf("invalid user handle")
		}

		user, err := hdl.users.GetByID(r.Context(), userID)
		if err != nil {
			return nil, fmt.Errorf("user not found")
		}

		passkeys, err := hdl.passkeys.GetByUserID(r.Context(), user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get passkeys")
		}

		return auth.NewWebAuthnUser(user, passkeys), nil
	})

	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to finish login", "err", err)
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Authentication failed"})
		return
	}

	// Find the passkey that was used
	passkey, err := hdl.passkeys.GetByCredentialID(r.Context(), credential.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to find passkey", "err", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Passkey not found"})
		return
	}

	// Update sign count
	if err := hdl.passkeys.UpdateSignCount(r.Context(), passkey.ID, credential.Authenticator.SignCount); err != nil {
		slog.ErrorContext(r.Context(), "Failed to update sign count", "err", err)
	}

	// Get the user
	user, err := hdl.users.GetByID(r.Context(), passkey.UserID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get user", "err", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "User not found"})
		return
	}

	// Create session using authboss
	// We need to set the user in the session manually
	session, err := hdl.sessionStore.Get(r, "probably_session")
	if err != nil {
		slog.WarnContext(r.Context(), "session cookie invalid, using fresh session", "err", err)
	}
	session.Values["uid"] = user.Email // Authboss uses email as PID
	if err := session.Save(r, w); err != nil {
		slog.ErrorContext(r.Context(), "Failed to save session", "err", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create session"})
		return
	}

	slog.InfoContext(r.Context(), "user logged in via passkey", "email", user.Email)

	// Return success with redirect
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/intelligence"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"redirect": redirect,
	})
}

// DeletePasskey removes a passkey
func (hdl *Handlers) DeletePasskey(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	passkeyID, ok := mustParamUUID(w, r, "id", "passkey ID")
	if !ok {
		return
	}

	// Verify ownership
	passkey, err := hdl.passkeys.GetByID(r.Context(), passkeyID)
	if err != nil {
		http.Error(w, "Passkey not found", http.StatusNotFound)
		return
	}

	if passkey.UserID != user.ID {
		http.Error(w, "Passkey not found", http.StatusNotFound)
		return
	}

	// Delete the passkey
	if err := hdl.passkeys.Delete(r.Context(), passkeyID); err != nil {
		http.Error(w, "Failed to delete passkey", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "user deleted passkey", "email", user.Email, "passkey", passkey.Name)

	// Redirect back to security settings
	http.Redirect(w, r, "/settings/security", http.StatusSeeOther)
}

// SettingsSecurity shows the security settings page with passkey management
func (hdl *Handlers) SettingsSecurity(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	// Get user's passkeys
	passkeys, err := hdl.passkeys.GetByUserID(r.Context(), user.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get passkeys", "err", err)
		passkeys = []*models.Passkey{}
	}

	// Check for success/error messages
	success := r.URL.Query().Get("success")
	errorMsg := r.URL.Query().Get("error")

	page := layouts.SettingsLayout("Security", user.Email, "security", user.ID.String(),
		shadcn.PageHeader("Security", "Manage your account security settings"),

		hx.Div(
			hx.Class("space-y-6"),

			// Success/error alerts
			g.If(success == "passkey_registered",
				shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertInfo},
					shadcn.AlertDescription(g.Text("Passkey registered successfully!")),
				),
			),
			g.If(success == "passkey_deleted",
				shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertInfo},
					shadcn.AlertDescription(g.Text("Passkey deleted successfully.")),
				),
			),
			g.If(errorMsg != "",
				shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertDestructive},
					shadcn.AlertDescription(g.Text(errorMsg)),
				),
			),

			// Passkeys section
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardHeaderActions("Passkeys", "Use your device's biometric authentication or security key to sign in without a password."),
				shadcn.CardContentFull(
					hx.Div(
						hx.Class("space-y-4"),

						// Info about passkeys
						hx.P(
							hx.Class("text-sm text-muted-foreground"),
							g.Text("Passkeys are a more secure and convenient alternative to passwords. They use your device's built-in biometric authentication (Face ID, Touch ID, Windows Hello) or a hardware security key."),
						),

						// Existing passkeys
						g.If(len(passkeys) > 0,
							hx.Div(
								hx.Class("space-y-2 mt-4"),
								hx.H4(hx.Class("text-sm font-medium text-foreground"), g.Text("Your passkeys")),
								hx.Div(
									hx.Class("divide-y divide-border border border-border rounded-lg"),
									g.Group(g.Map(passkeys, func(p *models.Passkey) g.Node {
										return renderPasskeyRow(p)
									})),
								),
							),
						),

						g.If(len(passkeys) == 0,
							hx.P(
								hx.Class("text-sm text-muted-foreground italic mt-4"),
								g.Text("You haven't registered any passkeys yet."),
							),
						),

						// Register new passkey button
						hx.Div(
							hx.Class("mt-4"),
							hx.Button(
								hx.Type("button"),
								hx.ID("register-passkey-btn"),
								hx.Class("inline-flex items-center gap-2 bg-primary text-primary-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:opacity-90 transition-colors"),
								g.Raw(`<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>`),
								g.Text("Register new passkey"),
							),
						),
					),
				),
			),

			// Password section
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardHeaderActions("Password", ""),
				shadcn.CardContentFull(
					hx.Form(
						hx.Method("POST"),
						hx.Action("/settings/password"),
						hx.Input(hx.Type("hidden"), hx.Name("_method"), hx.Value("PUT")),
						hx.Class("space-y-4"),

						shadcn.FormField(shadcn.FormFieldProps{Name: "current_password"},
							shadcn.Label(shadcn.LabelProps{For: "current_password", Required: true},
								g.Text("Current Password"),
							),
							shadcn.Input(shadcn.InputProps{
								Type:     "password",
								Name:     "current_password",
								Required: true,
							}),
						),

						shadcn.FormField(shadcn.FormFieldProps{Name: "new_password"},
							shadcn.Label(shadcn.LabelProps{For: "new_password", Required: true},
								g.Text("New Password"),
							),
							shadcn.Input(shadcn.InputProps{
								Type:      "password",
								Name:      "new_password",
								Required:  true,
								MinLength: 8,
							}),
						),

						shadcn.FormField(shadcn.FormFieldProps{Name: "confirm_password"},
							shadcn.Label(shadcn.LabelProps{For: "confirm_password", Required: true},
								g.Text("Confirm New Password"),
							),
							shadcn.Input(shadcn.InputProps{
								Type:     "password",
								Name:     "confirm_password",
								Required: true,
							}),
						),

						shadcn.Button(shadcn.ButtonProps{
							Variant: shadcn.ButtonDefault,
							Type:    "submit",
						},
							g.Text("Change Password"),
						),
					),
				),
			),
		),

		// JavaScript for WebAuthn registration
		hx.Script(hx.Type("text/javascript"), g.Raw(passkeyRegistrationScript)),
	)

	renderHTML(w, page)
}

// renderPasskeyRow renders a single passkey row
func renderPasskeyRow(p *models.Passkey) g.Node {
	lastUsed := "Never used"
	if p.LastUsedAt != nil {
		lastUsed = "Last used " + formatRelativeTime(*p.LastUsedAt)
	}

	return hx.Div(
		hx.Class("flex items-center justify-between p-4"),
		hx.Div(
			hx.Class("flex items-center gap-3"),
			hx.Div(
				hx.Class("w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground"),
				g.Raw(`<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"></path></svg>`),
			),
			hx.Div(
				hx.P(hx.Class("font-medium text-foreground"), g.Text(p.Name)),
				hx.P(hx.Class("text-sm text-muted-foreground"), g.Text(lastUsed)),
				hx.P(hx.Class("text-xs text-muted-foreground"), g.Text("Registered "+p.CreatedAt.Format("Jan 2, 2006"))),
			),
		),
		hx.Form(
			hx.Method("POST"),
			hx.Action(fmt.Sprintf("/settings/security/passkeys/%s", p.ID)),
			hx.Input(hx.Type("hidden"), hx.Name("_method"), hx.Value("DELETE")),
			hx.Button(
				hx.Type("submit"),
				hx.Class("text-destructive hover:opacity-80 text-sm"),
				g.Attr("onclick", "return confirm('Delete this passkey? You will no longer be able to sign in with it.')"),
				g.Text("Remove"),
			),
		),
	)
}

// JavaScript for passkey registration
const passkeyRegistrationScript = `
(function() {
	const registerBtn = document.getElementById('register-passkey-btn');
	if (!registerBtn) return;

	// Check if WebAuthn is supported
	if (!window.PublicKeyCredential) {
		registerBtn.disabled = true;
		registerBtn.textContent = 'Passkeys not supported in this browser';
		registerBtn.classList.add('opacity-50', 'cursor-not-allowed');
		return;
	}

	registerBtn.addEventListener('click', async function() {
		try {
			registerBtn.disabled = true;
			registerBtn.textContent = 'Waiting for device...';

			// Get registration options from server
			const optionsRes = await fetch('/auth/passkey/register/begin', {
				method: 'POST',
				credentials: 'same-origin',
			});

			if (!optionsRes.ok) {
				throw new Error('Failed to get registration options');
			}

			const options = await optionsRes.json();

			// Convert base64url to ArrayBuffer
			options.publicKey.challenge = base64urlToBuffer(options.publicKey.challenge);
			options.publicKey.user.id = base64urlToBuffer(options.publicKey.user.id);
			if (options.publicKey.excludeCredentials) {
				options.publicKey.excludeCredentials = options.publicKey.excludeCredentials.map(cred => ({
					...cred,
					id: base64urlToBuffer(cred.id)
				}));
			}

			// Create credential
			const credential = await navigator.credentials.create(options);

			// Prompt for passkey name
			let name = prompt('Name this passkey (e.g., "MacBook Touch ID", "iPhone")', '');
			if (!name) {
				name = 'Passkey ' + new Date().toLocaleDateString();
			}

			// Prepare credential for server
			const attestationObject = bufferToBase64url(credential.response.attestationObject);
			const clientDataJSON = bufferToBase64url(credential.response.clientDataJSON);

			// Send credential to server
			const finishRes = await fetch('/auth/passkey/register/finish?name=' + encodeURIComponent(name), {
				method: 'POST',
				credentials: 'same-origin',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({
					id: credential.id,
					rawId: bufferToBase64url(credential.rawId),
					type: credential.type,
					response: {
						attestationObject: attestationObject,
						clientDataJSON: clientDataJSON,
					},
				}),
			});

			const result = await finishRes.json();

			if (result.success) {
				window.location.href = '/settings/security?success=passkey_registered';
			} else {
				throw new Error(result.error || 'Registration failed');
			}
		} catch (err) {
			console.error('Passkey registration error:', err);
			alert('Failed to register passkey: ' + err.message);
			registerBtn.disabled = false;
			registerBtn.innerHTML = '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg> Register new passkey';
		}
	});

	// Helper functions
	function base64urlToBuffer(base64url) {
		const base64 = base64url.replace(/-/g, '+').replace(/_/g, '/');
		const padding = '='.repeat((4 - base64.length % 4) % 4);
		const binary = atob(base64 + padding);
		const buffer = new ArrayBuffer(binary.length);
		const view = new Uint8Array(buffer);
		for (let i = 0; i < binary.length; i++) {
			view[i] = binary.charCodeAt(i);
		}
		return buffer;
	}

	function bufferToBase64url(buffer) {
		const bytes = new Uint8Array(buffer);
		let binary = '';
		for (let i = 0; i < bytes.length; i++) {
			binary += String.fromCharCode(bytes[i]);
		}
		const base64 = btoa(binary);
		return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
	}
})();
`

// HasPasskeys checks if a user has any passkeys registered
func (hdl *Handlers) HasPasskeys(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		respondJSON(w, http.StatusOK, map[string]bool{"has_passkeys": false})
		return
	}

	user, err := hdl.users.GetByEmail(r.Context(), email)
	if err != nil {
		// Don't reveal if user exists
		respondJSON(w, http.StatusOK, map[string]bool{"has_passkeys": false})
		return
	}

	count, err := hdl.passkeys.CountByUserID(r.Context(), user.ID)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]bool{"has_passkeys": false})
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"has_passkeys": count > 0})
}
