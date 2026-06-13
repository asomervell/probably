package pages

import (
	"bytes"

	"github.com/aarondl/authboss/v3"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func RenderLogin(data authboss.HTMLData) ([]byte, error) {
	var errMsg string
	if errs, ok := data["errors"].([]string); ok && len(errs) > 0 {
		errMsg = errs[0]
	}

	csrfToken, _ := data["csrf_token"].(string)
	
	// Get redirect parameter from query string (passed from OAuth flow)
	// We'll need to get it from the request, but since we only have data here,
	// we'll check if it's in the data map (Authboss might pass it)
	redirectParam := ""
	if redir, ok := data["redirect"].(string); ok {
		redirectParam = redir
	}

	page := layouts.AuthLayout("Login",
		h.H2(
			h.Class("text-xl font-semibold text-white mb-6"),
			g.Text("Welcome back"),
		),

		g.If(errMsg != "", h.Div(h.Class("mb-4"), shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertDestructive}, shadcn.AlertDescription(g.Text(errMsg))))),

		// Passkey login button
		h.Div(
			h.ID("passkey-login-section"),
			h.Class("mb-6"),
			h.Button(
				h.Type("button"),
				h.ID("passkey-login-btn"),
				h.Class("w-full flex items-center justify-center gap-2 bg-secondary text-secondary-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-colors border border-border"),
				g.Raw(`<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"></path></svg>`),
				g.Text("Sign in with passkey"),
			),
			h.Div(
				h.Class("relative my-4"),
				h.Div(h.Class("absolute inset-0 flex items-center"),
					h.Div(h.Class("w-full border-t border-border")),
				),
				h.Div(h.Class("relative flex justify-center text-xs uppercase"),
					h.Span(h.Class("bg-card px-2 text-muted-foreground"), g.Text("Or continue with password")),
				),
			),
		),

		h.Form(
			h.Method("POST"),
			h.Action("/auth/login"),
			h.Class("space-y-4"),
			h.ID("login-form"),

			// CSRF
			h.Input(h.Type("hidden"), h.Name("csrf_token"), h.Value(csrfToken)),
			
			// Preserve redirect parameter through POST (will be set by JavaScript)
			h.Input(h.Type("hidden"), h.Name("redirect"), h.ID("redirect-input"), h.Value(redirectParam)),

			shadcn.FormField(shadcn.FormFieldProps{Name: "email"},
				shadcn.Label(shadcn.LabelProps{For: "email"}, g.Text("Email")),
				shadcn.Input(shadcn.InputProps{
					Type:        "email",
					Name:        "email",
					ID:          "email",
					Placeholder: "you@example.com",
					Required:    true,
				}, h.AutoComplete("email")),
			),

			shadcn.FormField(shadcn.FormFieldProps{Name: "password"},
				shadcn.Label(shadcn.LabelProps{For: "password"}, g.Text("Password")),
				shadcn.Input(shadcn.InputProps{
					Type:        "password",
					Name:        "password",
					ID:          "password",
					Placeholder: "••••••••",
					Required:    true,
				}, h.AutoComplete("current-password")),
			),

			h.Div(
				h.Class("flex items-center justify-between"),
				h.Label(
					h.Class("flex items-center gap-2 cursor-pointer"),
					h.Input(
						h.Type("checkbox"),
						h.Name("rm"),
						h.Class("w-4 h-4 rounded bg-input border-border text-primary focus:ring-ring focus:ring-offset-background"),
					),
					h.Span(h.Class("text-sm text-muted-foreground"), g.Text("Remember me")),
				),
				h.A(
					h.Href("/auth/recover"),
					h.Class("text-sm text-primary hover:opacity-80"),
					g.Text("Forgot password?"),
				),
			),

			h.Div(
				h.Class("pt-2"),
				h.Button(
					h.Type("submit"),
					h.Class("w-full bg-primary text-primary-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-colors"),
					g.Text("Sign in"),
				),
			),
		),

		h.P(
			h.Class("mt-6 text-center text-sm text-muted-foreground"),
			g.Text("Don't have an account? "),
			h.A(
				h.Href("/auth/register"),
				h.Class("text-primary hover:opacity-80 font-medium"),
				g.Text("Create one"),
			),
		),
		
		// JavaScript for passkey login and redirect preservation
		h.Script(h.Type("text/javascript"),
			g.Raw(passkeyLoginScript),
		),
	)

	var buf bytes.Buffer
	if err := page.Render(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// passkeyLoginScript handles passkey authentication on the login page
const passkeyLoginScript = `
(function() {
	// Preserve redirect parameter from URL
	const urlParams = new URLSearchParams(window.location.search);
	const redirect = urlParams.get('redirect') || urlParams.get('redir');
	if (redirect) {
		const input = document.getElementById('redirect-input');
		if (input) {
			input.value = redirect;
		}
	}

	// Check if WebAuthn is supported
	const passkeyBtn = document.getElementById('passkey-login-btn');
	const passkeySection = document.getElementById('passkey-login-section');
	
	if (!window.PublicKeyCredential) {
		// Hide passkey section if not supported
		if (passkeySection) {
			passkeySection.style.display = 'none';
		}
		return;
	}

	// Passkey login handler
	if (passkeyBtn) {
		passkeyBtn.addEventListener('click', async function() {
			try {
				passkeyBtn.disabled = true;
				passkeyBtn.innerHTML = '<svg class="w-5 h-5 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg> Waiting for passkey...';

				// Start discoverable login (no email needed)
				const optionsRes = await fetch('/auth/passkey/login/begin', {
					method: 'POST',
					credentials: 'same-origin',
				});

				if (!optionsRes.ok) {
					const err = await optionsRes.json();
					throw new Error(err.error || 'Failed to start passkey login');
				}

				const options = await optionsRes.json();

				// Convert base64url to ArrayBuffer
				options.publicKey.challenge = base64urlToBuffer(options.publicKey.challenge);
				if (options.publicKey.allowCredentials) {
					options.publicKey.allowCredentials = options.publicKey.allowCredentials.map(cred => ({
						...cred,
						id: base64urlToBuffer(cred.id)
					}));
				}

				// Get credential
				const credential = await navigator.credentials.get(options);

				// Prepare for server
				const finishRes = await fetch('/auth/passkey/login/finish' + (redirect ? '?redirect=' + encodeURIComponent(redirect) : ''), {
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
							authenticatorData: bufferToBase64url(credential.response.authenticatorData),
							clientDataJSON: bufferToBase64url(credential.response.clientDataJSON),
							signature: bufferToBase64url(credential.response.signature),
							userHandle: credential.response.userHandle ? bufferToBase64url(credential.response.userHandle) : null,
						},
					}),
				});

				const result = await finishRes.json();

				if (result.success) {
					window.location.href = result.redirect || '/intelligence';
				} else {
					throw new Error(result.error || 'Authentication failed');
				}
			} catch (err) {
				console.error('Passkey login error:', err);
				if (err.name !== 'NotAllowedError') {
					alert('Passkey login failed: ' + err.message);
				}
				passkeyBtn.disabled = false;
				passkeyBtn.innerHTML = '<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"></path></svg> Sign in with passkey';
			}
		});
	}

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

func RenderRegister(data authboss.HTMLData) ([]byte, error) {
	var errMsg string
	if errs, ok := data["errors"].([]string); ok && len(errs) > 0 {
		errMsg = errs[0]
	}

	csrfToken, _ := data["csrf_token"].(string)

	page := layouts.AuthLayout("Create Account",
		h.H2(
			h.Class("text-xl font-semibold text-white mb-4"),
			g.Text("Create your account"),
		),

		h.Div(
			h.Class("mb-6 p-4 bg-primary/10 border border-primary/30 rounded-lg"),
			h.P(h.Class("text-sm font-medium text-primary mb-1"), g.Text("45-Day Free Trial")),
			h.P(h.Class("text-sm text-primary/90"), 
				g.Text("Start with a 45-day free trial. We want you to experience a full month to really understand how game-changing Probably is."),
			),
		),

		g.If(errMsg != "", h.Div(h.Class("mb-4"), shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertDestructive}, shadcn.AlertDescription(g.Text(errMsg))))),

		h.Form(
			h.Method("POST"),
			h.Action("/auth/register"),
			h.Class("space-y-4"),

			// CSRF
			h.Input(h.Type("hidden"), h.Name("csrf_token"), h.Value(csrfToken)),

			shadcn.FormField(shadcn.FormFieldProps{Name: "email"},
				shadcn.Label(shadcn.LabelProps{For: "email"}, g.Text("Email")),
				shadcn.Input(shadcn.InputProps{
					Type:        "email",
					Name:        "email",
					ID:          "email",
					Placeholder: "you@example.com",
					Required:    true,
				}, h.AutoComplete("email")),
			),

			shadcn.FormField(shadcn.FormFieldProps{Name: "password"},
				shadcn.Label(shadcn.LabelProps{For: "password"}, g.Text("Password")),
				shadcn.Input(shadcn.InputProps{
					Type:        "password",
					Name:        "password",
					ID:          "password",
					Placeholder: "Choose a strong password",
					Required:    true,
					MinLength:   8,
				}, h.AutoComplete("new-password")),
			),

			shadcn.FormField(shadcn.FormFieldProps{Name: "confirm_password"},
				shadcn.Label(shadcn.LabelProps{For: "confirm_password"}, g.Text("Confirm Password")),
				shadcn.Input(shadcn.InputProps{
					Type:        "password",
					Name:        "confirm_password",
					ID:          "confirm_password",
					Placeholder: "Repeat your password",
					Required:    true,
				}, h.AutoComplete("new-password")),
			),

			h.Div(
				h.Class("pt-2"),
				h.Button(
					h.Type("submit"),
					h.Class("w-full bg-primary text-primary-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-colors"),
					g.Text("Create account"),
				),
			),
		),

		h.P(
			h.Class("mt-6 text-center text-sm text-muted-foreground"),
			g.Text("Already have an account? "),
			h.A(
				h.Href("/auth/login"),
				h.Class("text-primary hover:opacity-80 font-medium"),
				g.Text("Sign in"),
			),
		),
	)

	var buf bytes.Buffer
	if err := page.Render(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func RenderRecoverStart(data authboss.HTMLData) ([]byte, error) {
	var errMsg string
	if errs, ok := data["errors"].([]string); ok && len(errs) > 0 {
		errMsg = errs[0]
	}

	csrfToken, _ := data["csrf_token"].(string)

	page := layouts.AuthLayout("Reset Password",
		h.H2(
			h.Class("text-xl font-semibold text-white mb-2"),
			g.Text("Reset your password"),
		),
		h.P(
			h.Class("text-muted-foreground mb-6"),
			g.Text("Enter your email and we'll send you a link to reset your password."),
		),

		g.If(errMsg != "", h.Div(h.Class("mb-4"), shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertDestructive}, shadcn.AlertDescription(g.Text(errMsg))))),

		h.Form(
			h.Method("POST"),
			h.Action("/auth/recover"),
			h.Class("space-y-4"),

			// CSRF
			h.Input(h.Type("hidden"), h.Name("csrf_token"), h.Value(csrfToken)),

			shadcn.FormField(shadcn.FormFieldProps{Name: "email"},
				shadcn.Label(shadcn.LabelProps{For: "email"}, g.Text("Email")),
				shadcn.Input(shadcn.InputProps{
					Type:        "email",
					Name:        "email",
					ID:          "email",
					Placeholder: "you@example.com",
					Required:    true,
				}, h.AutoComplete("email")),
			),

			h.Div(
				h.Class("pt-2"),
				h.Button(
					h.Type("submit"),
					h.Class("w-full bg-primary text-primary-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-colors"),
					g.Text("Send reset link"),
				),
			),
		),

		h.P(
			h.Class("mt-6 text-center text-sm text-muted-foreground"),
			g.Text("Remember your password? "),
			h.A(
				h.Href("/auth/login"),
				h.Class("text-primary hover:opacity-80 font-medium"),
				g.Text("Sign in"),
			),
		),
	)

	var buf bytes.Buffer
	if err := page.Render(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func RenderRecoverEnd(data authboss.HTMLData) ([]byte, error) {
	var errMsg string
	if errs, ok := data["errors"].([]string); ok && len(errs) > 0 {
		errMsg = errs[0]
	}

	csrfToken, _ := data["csrf_token"].(string)
	token, _ := data["recovery_token"].(string)

	page := layouts.AuthLayout("Set New Password",
		h.H2(
			h.Class("text-xl font-semibold text-white mb-6"),
			g.Text("Set your new password"),
		),

		g.If(errMsg != "", h.Div(h.Class("mb-4"), shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertDestructive}, shadcn.AlertDescription(g.Text(errMsg))))),

		h.Form(
			h.Method("POST"),
			h.Action("/auth/recover/end"),
			h.Class("space-y-4"),

			// CSRF and token
			h.Input(h.Type("hidden"), h.Name("csrf_token"), h.Value(csrfToken)),
			h.Input(h.Type("hidden"), h.Name("token"), h.Value(token)),

			shadcn.FormField(shadcn.FormFieldProps{Name: "password"},
				shadcn.Label(shadcn.LabelProps{For: "password"}, g.Text("New Password")),
				shadcn.Input(shadcn.InputProps{
					Type:        "password",
					Name:        "password",
					ID:          "password",
					Placeholder: "Choose a strong password",
					Required:    true,
					MinLength:   8,
				}, h.AutoComplete("new-password")),
			),

			shadcn.FormField(shadcn.FormFieldProps{Name: "confirm_password"},
				shadcn.Label(shadcn.LabelProps{For: "confirm_password"}, g.Text("Confirm Password")),
				shadcn.Input(shadcn.InputProps{
					Type:        "password",
					Name:        "confirm_password",
					ID:          "confirm_password",
					Placeholder: "Repeat your password",
					Required:    true,
				}, h.AutoComplete("new-password")),
			),

			h.Div(
				h.Class("pt-2"),
				h.Button(
					h.Type("submit"),
					h.Class("w-full bg-primary text-primary-foreground rounded-lg px-4 py-2.5 text-sm font-medium hover:opacity-90 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-colors"),
					g.Text("Reset password"),
				),
			),
		),
	)

	var buf bytes.Buffer
	if err := page.Render(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

