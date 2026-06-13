package layouts

import (
	"encoding/json"
	"fmt"

	"github.com/asomervell/probably/internal/config"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// PostHog view bootstrap (set once at process start from server config).
var ph struct {
	enabled          bool
	projectKey       string
	apiHost          string
	sessionReplay    bool
	autocapture      bool
	capturePageview  bool
}

// InitPostHogFromConfig configures the HTML snippet (browser SDK). Safe to call once at startup.
func InitPostHogFromConfig(cfg *config.Config) {
	if cfg.PostHogDisableWeb || cfg.PostHogProjectAPIKey == "" {
		ph.enabled = false
		return
	}
	ph.enabled = true
	ph.projectKey = cfg.PostHogProjectAPIKey
	ph.apiHost = cfg.PostHogAPIHost
	if ph.apiHost == "" {
		ph.apiHost = "https://us.i.posthog.com"
	}
	ph.sessionReplay = cfg.PostHogSessionReplay
	ph.autocapture = cfg.PostHogAutocapture
	ph.capturePageview = cfg.PostHogCapturePageview
}

// posthogHeadFragment returns empty content when PostHog is disabled.
func posthogHeadFragment(distinctID string) g.Node {
	return g.Group(posthogScriptNodes(distinctID))
}

func posthogScriptNodes(distinctID string) []g.Node {
	if !ph.enabled {
		return nil
	}
	keyJSON, err := json.Marshal(ph.projectKey)
	if err != nil {
		return nil
	}
	opts := map[string]any{
		"api_host":            ph.apiHost,
		"defaults":            "2025-11-30",
		"capture_pageview":    ph.capturePageview,
		"capture_pageleave":   true,
		"autocapture":         ph.autocapture,
		"disable_session_recording": !ph.sessionReplay,
		"person_profiles":     "identified_only",
	}
	optsJSON, err := json.Marshal(opts)
	if err != nil {
		return nil
	}
	loader := `!function(t,e){var o,n,p,r;e.__SV||(window.posthog=e,e._i=[],e.init=function(i,s,a){function g(t,e){var o=e.split(".");2==o.length&&(t=t[o[0]],e=o[1]),t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}}(p=t.createElement("script")).type="text/javascript",p.crossOrigin="anonymous",p.async=!0,p.src=s.api_host+"/static/array.js",(r=t.getElementsByTagName("script")[0]).parentNode.insertBefore(p,r);var u=e;for(void 0!==a?u=e[a]=[]:a="posthog",u.people=u.people||[],u.toString=function(t){var e="posthog";return"posthog"!==a&&(e+="."+a),t||(e+=" (stub)"),e},u.people.toString=function(){return u.toString(1)+".people (stub)"},o="capture identify alias people.set people.set_once set_config register register_once unregister opt_out_capturing has_opted_out_capturing opt_in_capturing reset isFeatureEnabled onFeatureFlags getFeatureFlag getFeatureFlagPayload reloadFeatureFlags group updateEarlyAccessEnrollmentEnrollment getEarlyAccessKeystate getActiveMatchingSurveys getSurveys onSessionId".split(" "),n=0;n<o.length;n++)g(u,o[n]);e._i.push([i,s,a])},e.__SV=1)}(document,window.posthog||[]);`

	init := fmt.Sprintf("%sposthog.init(%s,%s);", loader, string(keyJSON), string(optsJSON))

	var identify string
	if distinctID != "" {
		dj, _ := json.Marshal(distinctID)
		identify = fmt.Sprintf("if(window.posthog&&posthog.identify){posthog.identify(%s);}", string(dj))
	}

	htmxNav := `
document.body.addEventListener('htmx:afterSettle', function() {
	if (window.posthog && typeof posthog.capture === 'function') {
		posthog.capture('$pageview', { $current_url: window.location.href });
	}
});
`

	globalErrors := `
(function() {
	function safeCapture(errorLike, properties) {
		if (!window.posthog) return;
		if (typeof window.posthog.captureException === 'function') {
			window.posthog.captureException(errorLike, properties || {});
			return;
		}
		if (typeof window.posthog.capture === 'function') {
			var message = 'Unknown frontend exception';
			if (errorLike && errorLike.message) {
				message = errorLike.message;
			} else if (typeof errorLike === 'string') {
				message = errorLike;
			}
			window.posthog.capture('frontend_exception', {
				message: message,
				...properties,
			});
		}
	}

	window.addEventListener('error', function(event) {
		safeCapture(event.error || event.message || 'window.error', {
			source: 'window.error',
			filename: event.filename || '',
			lineno: String(event.lineno || ''),
			colno: String(event.colno || ''),
			$current_url: window.location.href,
		});
	});

	window.addEventListener('unhandledrejection', function(event) {
		var reason = event.reason;
		var errorLike = reason;
		if (!(reason instanceof Error)) {
			try {
				errorLike = new Error(typeof reason === 'string' ? reason : JSON.stringify(reason));
			} catch (_) {
				errorLike = new Error('Unhandled promise rejection');
			}
		}
		safeCapture(errorLike, {
			source: 'window.unhandledrejection',
			$current_url: window.location.href,
		});
	});
})();
`

	return []g.Node{
		h.Script(g.Raw(init + identify)),
		h.Script(g.Raw(htmxNav)),
		h.Script(g.Raw(globalErrors)),
	}
}
