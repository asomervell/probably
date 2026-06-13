package pages

import (
	"fmt"
	"time"

	"github.com/asomervell/probably/internal/views/layouts"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// ThreadListItem represents a thread for rendering
type ThreadListItem struct {
	ID        string
	Title     string
	UpdatedAt time.Time
	IsActive  bool
}

// ChatMessage represents a message for rendering
type ChatMessage struct {
	ID      string
	Role    string
	Content string
}

// RenderChat renders the chat interface page
func RenderChat(userEmail, posthogDistinctID, ledgerID, threadID, threadTitle string, messages []ChatMessage) g.Node {
	return layouts.AppLayout("AI Chat", userEmail, posthogDistinctID, renderChatContent(ledgerID, threadID, threadTitle, messages))
}

// RenderThreadList renders the thread list as an HTML partial (for HTMX)
func RenderThreadList(threads []ThreadListItem) g.Node {
	if len(threads) == 0 {
		return h.Div(
			h.Class("text-center text-muted-foreground py-8"),
			g.Text("No conversations yet"),
		)
	}

	var items []g.Node
	for _, t := range threads {
		items = append(items, renderThreadItem(t))
	}
	return g.Group(items)
}

func renderThreadItem(t ThreadListItem) g.Node {
	activeClass := ""
	if t.IsActive {
		activeClass = "bg-primary/20 border border-primary/30"
	}

	return h.Div(
		h.Class(fmt.Sprintf("group relative p-3 rounded-lg transition-colors cursor-pointer hover:bg-accent %s", activeClass)),
		// Click to load thread (HTMX)
		g.Attr("hx-get", fmt.Sprintf("/chat/threads/%s/load", t.ID)),
		g.Attr("hx-target", "#chat-messages"),
		g.Attr("hx-push-url", fmt.Sprintf("/chat?t=%s", t.ID)),
		g.Attr("hx-on::after-request", "closeDrawer(); document.getElementById('chat-suggestions').style.display='none';"),

		h.Div(
			h.Class("flex items-start justify-between gap-2"),
			h.Div(
				h.Class("flex-1 min-w-0"),
				h.Div(
					h.Class("text-sm font-medium text-foreground truncate"),
					g.Text(t.Title),
				),
				h.Div(
					h.Class("text-xs text-muted-foreground mt-0.5"),
					g.Text(formatRelativeTime(t.UpdatedAt)),
				),
			),
			// Delete button
			h.Button(
				h.Type("button"),
				h.Class("opacity-0 group-hover:opacity-100 p-1 rounded text-muted-foreground hover:text-destructive hover:bg-accent transition-all"),
				g.Attr("hx-delete", fmt.Sprintf("/chat/threads/%s", t.ID)),
				g.Attr("hx-target", "closest .group"),
				g.Attr("hx-swap", "outerHTML"),
				g.Attr("hx-confirm", "Delete this conversation?"),
				g.Attr("onclick", "event.stopPropagation()"),
				g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>`),
			),
		),
	)
}

// RenderChatMessages renders messages as HTML partial (for HTMX)
func RenderChatMessages(messages []ChatMessage, threadID, threadTitle string) g.Node {
	var nodes []g.Node

	// Update the title via out-of-band swap
	nodes = append(nodes, h.Span(
		h.ID("chat-title"),
		g.Attr("hx-swap-oob", "innerHTML"),
		g.Text(threadTitle),
	))

	// Update hidden thread ID input
	nodes = append(nodes, h.Input(
		h.ID("chat-thread-id"),
		h.Type("hidden"),
		h.Value(threadID),
		g.Attr("hx-swap-oob", "outerHTML"),
	))

	// Render messages
	for _, msg := range messages {
		nodes = append(nodes, RenderMessage(msg))
	}

	return g.Group(nodes)
}

func RenderMessage(msg ChatMessage) g.Node {
	if msg.Role == "user" {
		return h.Div(
			h.Class("user-message flex items-end gap-3 justify-end"),
			h.Div(
				h.Class("max-w-[80%] rounded-lg bg-primary text-primary-foreground px-4 py-3"),
				h.P(h.Class("text-sm"), g.Text(msg.Content)),
			),
			h.Div(
				h.Class("user-avatar flex h-8 w-8 items-center justify-center rounded-full bg-secondary shrink-0"),
				h.Span(h.Class("text-xs font-medium text-secondary-foreground"), g.Text("You")),
			),
		)
	}
	// Assistant message - use prose styling with proper spacing for markdown content
	return h.Div(
		h.Class("assistant-message"),
		h.Div(
			h.Class("prose prose-neutral dark:prose-invert max-w-none text-foreground leading-6 prose-p:my-3 prose-p:leading-6 prose-headings:text-foreground prose-strong:text-foreground prose-li:text-foreground prose-li:my-1 prose-ul:my-2 prose-ol:my-2"),
			g.Raw(msg.Content), // Content is pre-rendered HTML from markdown
		),
	)
}

func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < 24*time.Hour {
		return t.Format("3:04 PM")
	}
	if diff < 48*time.Hour {
		return "Yesterday"
	}
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}
	return t.Format("Jan 2")
}

// renderChatContent renders the chat interface (replicating shadcn.ai chatbot block)
func renderChatContent(ledgerID, threadID, threadTitle string, messages []ChatMessage) g.Node {
	title := threadTitle
	if title == "" {
		title = "New Chat"
	}

	// Pre-render messages if provided
	var messageNodes []g.Node
	for _, msg := range messages {
		messageNodes = append(messageNodes, RenderMessage(msg))
	}

	// Hide suggestions if we have messages
	suggestionsDisplay := "flex"
	if len(messages) > 0 {
		suggestionsDisplay = "none"
	}

	return h.Div(
		h.Class("flex flex-col min-h-0 h-[calc(100vh-8rem)] max-h-[50rem]"),
		// Header with history button
		h.Div(
			h.Class("flex items-center justify-between mb-4"),
			h.Div(
				h.Class("flex items-center gap-3"),
				// History button - loads thread list via HTMX
				h.Button(
					h.Type("button"),
					h.ID("history-btn"),
					h.Class("flex items-center justify-center h-9 w-9 rounded-lg bg-secondary text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"),
					h.Title("Chat History"),
					g.Attr("hx-get", "/chat/threads/list"),
					g.Attr("hx-target", "#thread-list"),
					g.Attr("hx-trigger", "click"),
					g.Attr("onclick", "openDrawer()"),
					g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>`),
				),
				h.H1(
					h.Class("text-lg font-medium text-foreground"),
					h.ID("chat-title"),
					g.Text(title),
				),
			),
			// New chat button - navigates to fresh chat
			h.A(
				h.Href("/chat"),
				h.Class("flex items-center gap-2 px-3 py-2 text-sm rounded-lg bg-secondary text-secondary-foreground hover:bg-accent transition-colors"),
				g.Attr("onclick", "closeDrawer()"),
				g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>`),
				g.Text("New"),
			),
		),

		// History drawer overlay (hidden by default)
		h.Div(
			h.ID("history-overlay"),
			h.Class("fixed inset-0 bg-black/50 z-40 hidden"),
			g.Attr("onclick", "closeDrawer()"),
		),

		// History drawer (slides from bottom on mobile, left on desktop)
		h.Div(
			h.ID("history-drawer"),
			h.Class("fixed z-50 bg-card border-border transition-transform duration-300 ease-out "+
				"bottom-0 left-0 right-0 h-[70vh] rounded-t-2xl border-t translate-y-full "+
				"md:top-0 md:bottom-0 md:right-auto md:h-full md:w-80 md:rounded-none md:border-r md:border-t-0 md:-translate-x-full md:translate-y-0"),
			// Drawer header
			h.Div(
				h.Class("flex items-center justify-between p-4 border-b border-border"),
				h.H2(
					h.Class("text-base font-medium text-foreground"),
					g.Text("Chat History"),
				),
				h.Button(
					h.Type("button"),
					h.ID("close-drawer-btn"),
					h.Class("flex items-center justify-center h-8 w-8 rounded-lg text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"),
					g.Attr("onclick", "closeDrawer()"),
					g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`),
				),
			),
			// Thread list container - populated by HTMX
			h.Div(
				h.ID("thread-list"),
				h.Class("flex-1 overflow-y-auto p-2"),
				// Loading indicator
				h.Div(
					h.Class("text-center text-muted-foreground py-4"),
					g.Text("Loading..."),
				),
			),
		),

		// Main chat container (for HTMX swapping)
		h.Div(
			h.ID("chat-container"),
			h.Class("flex-1 flex flex-col min-h-0"),
			// Chat messages (scrollable) - pre-rendered if thread loaded
			h.Div(
				h.ID("chat-messages"),
				h.Class("flex-1 overflow-y-auto space-y-4 mb-4 pr-2"),
				g.Group(messageNodes),
			),
		),

		// Suggestions container (hidden if thread has messages)
		h.Div(
			h.ID("chat-suggestions"),
			h.Class("flex flex-wrap gap-2 mb-3"),
			h.Style(fmt.Sprintf("display: %s", suggestionsDisplay)),
		),

		// Voice status indicator (hidden by default)
		h.Div(
			h.ID("voice-status"),
			h.Class("hidden mb-3 p-3 rounded-lg bg-secondary/50 border border-border"),
			h.Div(
				h.Class("flex items-center gap-3"),
				// Status icon (animated pulse when listening)
				h.Div(
					h.ID("voice-status-icon"),
					h.Class("flex h-8 w-8 items-center justify-center rounded-full bg-primary/20 shrink-0"),
					g.Raw(`<svg class="h-4 w-4 text-primary" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/></svg>`),
				),
				// Status text
				h.Div(
					h.Class("flex-1"),
					h.Div(
						h.ID("voice-status-text"),
						h.Class("text-sm font-medium text-foreground"),
						g.Text("Listening..."),
					),
					// Transcription preview
					h.Div(
						h.ID("voice-transcription"),
						h.Class("text-xs text-muted-foreground mt-1 italic"),
						g.Text(""),
					),
				),
				// Audio level indicator (waveform bars)
				h.Div(
					h.ID("voice-audio-level"),
					h.Class("flex items-end gap-1 h-8"),
					// Will be populated with animated bars
				),
				// Stop button (when recording)
				h.Button(
					h.Type("button"),
					h.ID("voice-stop-btn"),
					h.Class("hidden px-3 py-1.5 text-xs rounded-lg bg-destructive text-destructive-foreground hover:bg-destructive/90 transition-colors cursor-pointer"),
					h.Style("pointer-events: auto; z-index: 1000;"),
					g.Text("Stop"),
				),
			),
		),

		// Input form
		h.Form(
			h.ID("chat-form"),
			h.Class("flex items-end gap-2"),
			// Hidden ledger_id input
			h.Input(
				h.Type("hidden"),
				h.ID("chat-ledger-id"),
				h.Value(ledgerID),
			),
			// Hidden thread_id input
			h.Input(
				h.Type("hidden"),
				h.ID("chat-thread-id"),
				h.Value(threadID),
			),
			// Textarea
			h.Textarea(
				h.ID("chat-input"),
				h.Placeholder("Ask a question about your finances..."),
				h.Rows("1"),
				h.Class("flex-1 resize-none rounded-lg bg-input border border-border px-4 py-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:border-transparent"),
			),
			// Voice toggle button
			h.Button(
				h.Type("button"),
				h.ID("chat-voice-btn"),
				h.Class("flex h-10 w-10 items-center justify-center rounded-lg bg-secondary text-secondary-foreground hover:bg-accent hover:text-foreground transition-colors"),
				h.Title("Voice mode"),
				g.Raw(`<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/></svg>`),
			),
			// Send button
			h.Button(
				h.Type("button"),
				h.ID("chat-send"),
				h.Class("flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"),
				g.Raw(`<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/></svg>`),
			),
		),

		// Loading indicator (hidden by default)
		h.Div(
			h.ID("chat-loading"),
			h.Class("hidden flex items-start gap-3"),
			h.Div(
				h.Class("flex h-8 w-8 items-center justify-center rounded-full bg-secondary shrink-0"),
				h.Div(
					h.Class("h-2 w-2 rounded-full bg-primary animate-pulse"),
				),
			),
			h.Div(
				h.Class("flex-1 rounded-lg bg-card border border-border p-4"),
				h.Div(
					h.Class("flex gap-1"),
					h.Div(
						h.Class("h-2 w-2 rounded-full bg-muted-foreground animate-bounce"),
						h.Style("animation-delay: 0ms"),
					),
					h.Div(
						h.Class("h-2 w-2 rounded-full bg-muted-foreground animate-bounce"),
						h.Style("animation-delay: 150ms"),
					),
					h.Div(
						h.Class("h-2 w-2 rounded-full bg-muted-foreground animate-bounce"),
						h.Style("animation-delay: 300ms"),
					),
				),
			),
		),

		// Chart.js library (via CDN)
		h.Script(
			h.Src("https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"),
		),

		// JavaScript for drawer animation and SSE chat handling
		h.Script(g.Raw(`
			(function() {
				const chatMessages = document.getElementById('chat-messages');
				const chatInput = document.getElementById('chat-input');
				const chatSend = document.getElementById('chat-send');
				const ledgerID = document.getElementById('chat-ledger-id').value;
				const historyOverlay = document.getElementById('history-overlay');
				const historyDrawer = document.getElementById('history-drawer');

				// =====================================================
				// Drawer Animation (minimal JS for CSS transitions)
				// =====================================================
				
				window.openDrawer = function() {
					historyOverlay.classList.remove('hidden');
					requestAnimationFrame(() => {
						if (window.innerWidth >= 768) {
							historyDrawer.classList.remove('md:-translate-x-full');
						} else {
							historyDrawer.classList.remove('translate-y-full');
						}
					});
				};
				
				window.closeDrawer = function() {
					if (window.innerWidth >= 768) {
						historyDrawer.classList.add('md:-translate-x-full');
					} else {
						historyDrawer.classList.add('translate-y-full');
					}
					setTimeout(() => {
						historyOverlay.classList.add('hidden');
					}, 300);
				};

				// Auto-scroll to bottom when new messages arrive
				function scrollToBottom() {
					chatMessages.scrollTop = chatMessages.scrollHeight;
				}

				// Thinking indicator state
				let thinkingIndicatorId = null;
				let thinkingThoughts = [];

				// Create the thinking indicator component
				function createThinkingIndicator() {
					const id = 'thinking-' + Date.now();
					thinkingIndicatorId = id;
					thinkingThoughts = [];
					
					// Create wrapper container for both thinking and response
					const wrapper = document.createElement('div');
					wrapper.id = id + '-wrapper';
					wrapper.className = 'space-y-3';
					
					// Create thinking indicator with collapsible history
					const msg = document.createElement('div');
					msg.id = id;
					msg.className = 'thinking-container';
					msg.innerHTML = '' +
						'<div class="flex items-center gap-2 pt-1 pb-2 cursor-pointer" onclick="toggleThoughtHistory(\'' + id + '\')">' +
							'<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-primary shrink-0 animate-pulse" id="' + id + '-icon"><path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275z"/></svg>' +
							'<span id="' + id + '-current" class="text-sm text-muted-foreground animate-shimmer flex-1"></span>' +
							'<svg id="' + id + '-chevron" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted-foreground shrink-0 transition-transform duration-200"><path d="m6 9 6 6 6-6"/></svg>' +
						'</div>' +
						'<div id="' + id + '-history" class="hidden pl-5 ml-1.5 border-l border-border space-y-1 text-xs text-muted-foreground"></div>';
					
					// Create response container (hidden initially)
					const responseContainer = document.createElement('div');
					responseContainer.id = id + '-response';
					responseContainer.className = 'hidden';
					
					wrapper.appendChild(msg);
					wrapper.appendChild(responseContainer);
					chatMessages.appendChild(wrapper);
					scrollToBottom();
					return msg;
				}

				// Toggle thought history visibility
				window.toggleThoughtHistory = function(id) {
					const historyEl = document.getElementById(id + '-history');
					const chevronEl = document.getElementById(id + '-chevron');
					if (historyEl && chevronEl) {
						const isHidden = historyEl.classList.contains('hidden');
						if (isHidden) {
							historyEl.classList.remove('hidden');
							chevronEl.style.transform = 'rotate(180deg)';
						} else {
							historyEl.classList.add('hidden');
							chevronEl.style.transform = 'rotate(0deg)';
						}
					}
				};

				// Add a thought to the thinking indicator - updates current text and adds to history
				function addThought(thought) {
					if (!thinkingIndicatorId) return;
					
					const currentEl = document.getElementById(thinkingIndicatorId + '-current');
					const historyEl = document.getElementById(thinkingIndicatorId + '-history');
					if (!currentEl) return;
					
					// Add previous thought to history (if there was one)
					if (thinkingThoughts.length > 0 && historyEl) {
						const prevThought = thinkingThoughts[thinkingThoughts.length - 1];
						const historyItem = document.createElement('div');
						historyItem.textContent = prevThought;
						historyEl.appendChild(historyItem);
					}
					
					// Track thought
					thinkingThoughts.push(thought);
					
					// Update the current text with animation
					currentEl.classList.remove('thought-text-enter');
					void currentEl.offsetWidth; // Trigger reflow for animation restart
					currentEl.classList.add('thought-text-enter');
					currentEl.textContent = thought;
					
					scrollToBottom();
				}


				// Complete the thinking indicator with a summary message
				function completeThinkingIndicator(summary) {
					if (thinkingIndicatorId) {
						const el = document.getElementById(thinkingIndicatorId);
						const currentEl = document.getElementById(thinkingIndicatorId + '-current');
						const historyEl = document.getElementById(thinkingIndicatorId + '-history');
						const iconEl = document.getElementById(thinkingIndicatorId + '-icon');
						
						if (el) {
							// Add the last thought to history before showing summary
							if (thinkingThoughts.length > 0 && historyEl) {
								const lastThought = thinkingThoughts[thinkingThoughts.length - 1];
								const historyItem = document.createElement('div');
								historyItem.textContent = lastThought;
								historyEl.appendChild(historyItem);
							}
							
							// Stop the animations
							if (iconEl) {
								iconEl.classList.remove('animate-pulse');
								iconEl.classList.add('text-muted-foreground');
							}
							if (currentEl) {
								currentEl.classList.remove('animate-shimmer');
								currentEl.classList.add('text-muted-foreground');
								// Show the summary message
								currentEl.textContent = summary || 'Here\'s what I found:';
							}
						}
					}
				}

				// Get the response container for the current thinking indicator
				function getResponseContainer() {
					if (thinkingIndicatorId) {
						return document.getElementById(thinkingIndicatorId + '-response');
					}
					return null;
				}

				// Reset thinking state (called after response is added)
				function resetThinkingState() {
					thinkingIndicatorId = null;
					thinkingThoughts = [];
					lastMessageRole = null; // Reset message grouping after AI responds
				}

				// Remove the thinking indicator entirely (only used for errors)
				function removeThinkingIndicator() {
					if (thinkingIndicatorId) {
						const wrapper = document.getElementById(thinkingIndicatorId + '-wrapper');
						if (wrapper) wrapper.remove();
						thinkingIndicatorId = null;
						thinkingThoughts = [];
					}
				}

				// Track the last message role for grouping
				let lastMessageRole = null;

				// Add message to chat
				function addMessage(role, content, container) {
					const targetContainer = container || chatMessages;
					const msg = document.createElement('div');
					
					// Check if this continues a group from the same sender
					const isContinuation = (lastMessageRole === role);
					
					if (role === 'user') {
						// If continuing a user group, hide avatar from previous message
						if (isContinuation) {
							const prevAvatar = targetContainer.querySelector('.user-message:last-child .user-avatar');
							if (prevAvatar) prevAvatar.classList.add('invisible');
						}
						
						// User messages: right-aligned with avatar on the right, aligned to bottom
						msg.className = 'user-message flex items-end gap-3 justify-end';
						msg.innerHTML = '<div class="max-w-[80%] rounded-lg bg-primary text-primary-foreground px-4 py-3"><p class="text-sm">' + escapeHtml(content) + '</p></div><div class="user-avatar flex h-8 w-8 items-center justify-center rounded-full bg-secondary shrink-0"><span class="text-xs font-medium text-secondary-foreground">You</span></div>';
					} else if (role === 'assistant') {
						// Assistant messages: clean, no avatar, no background, with prose styling
						msg.className = 'assistant-message';
						msg.innerHTML = '<div class="prose prose-neutral dark:prose-invert max-w-none text-foreground leading-6 prose-p:my-3 prose-p:leading-6 prose-headings:text-foreground prose-strong:text-foreground prose-li:text-foreground prose-li:my-1 prose-ul:my-2 prose-ol:my-2">' + content + '</div>';
					} else if (role === 'loading') {
						msg.className = 'flex items-start gap-3';
						msg.innerHTML = '<div class="flex h-8 w-8 items-center justify-center rounded-full bg-secondary shrink-0"><div class="h-2 w-2 rounded-full bg-primary animate-pulse"></div></div><div class="flex-1 rounded-lg bg-card border border-border p-4"><div class="flex gap-1"><div class="h-2 w-2 rounded-full bg-muted-foreground animate-bounce" style="animation-delay: 0ms"></div><div class="h-2 w-2 rounded-full bg-muted-foreground animate-bounce" style="animation-delay: 150ms"></div><div class="h-2 w-2 rounded-full bg-muted-foreground animate-bounce" style="animation-delay: 300ms"></div></div></div>';
					} else if (role === 'error') {
						msg.className = 'flex items-start gap-3';
						msg.innerHTML = '<div class="flex h-8 w-8 items-center justify-center rounded-full bg-destructive shrink-0"><span class="text-sm font-medium text-destructive-foreground">!</span></div><div class="flex-1 rounded-lg bg-destructive/10 border border-destructive/30 text-destructive p-4"><p class="text-sm">' + escapeHtml(content) + '</p></div>';
					}
					
					// Update last message role (only for user/assistant, not thinking containers)
					if (role === 'user' || role === 'assistant') {
						lastMessageRole = role;
					}
					
					targetContainer.appendChild(msg);
					scrollToBottom();
					return msg;
				}

				// Escape HTML
				function escapeHtml(text) {
					const div = document.createElement('div');
					div.textContent = text;
					return div.innerHTML;
				}

				// Format numeric value - handles struct representations like {mantissa exponent ...}
				function parseNumericValue(val) {
					if (val === null || val === undefined) return null;
					
					// If already a number, return it
					if (typeof val === 'number') return val;
					
					const str = String(val).trim();
					if (str === '') return null;
					
					// Handle struct representation like {29124650000000000 -12 false finite true}
					if (str.startsWith('{') && str.includes('}')) {
						const content = str.slice(1, str.indexOf('}'));
						const parts = content.split(/\s+/);
						if (parts.length >= 2) {
							const mantissa = parseFloat(parts[0]);
							const exponent = parseFloat(parts[1]);
							if (!isNaN(mantissa) && !isNaN(exponent)) {
								return mantissa * Math.pow(10, exponent);
							}
						}
					}
					
					// Try parsing as regular number
					const num = parseFloat(str);
					if (!isNaN(num)) return num;
					
					return null;
				}
				
				// Format number as currency
				function formatCurrency(amount) {
					if (amount === null || isNaN(amount)) return '';
					const negative = amount < 0;
					const abs = Math.abs(amount);
					const formatted = '$' + abs.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
					return negative ? '-' + formatted : formatted;
				}
				
				// Format number with commas
				function formatNumber(num) {
					if (num === null || isNaN(num)) return '';
					if (num === Math.floor(num)) {
						return num.toLocaleString('en-US');
					}
					return num.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
				}
				
				// Format cell value based on column name and value type
				function formatCellValue(val, columnName) {
					if (val === null || val === undefined) return '';
					
					const colLower = String(columnName).toLowerCase();
					const isCurrencyColumn = colLower.includes('total') || colLower.includes('amount') || 
					                        colLower.includes('spent') || colLower.includes('spending') ||
					                        colLower.includes('cost') || colLower.includes('price') ||
					                        colLower.includes('balance') || colLower.includes('value');
					
					// Try to parse as number
					const num = parseNumericValue(val);
					if (num !== null) {
						return isCurrencyColumn ? formatCurrency(num) : formatNumber(num);
					}
					
					// Not a number, return as string
					return String(val);
				}

				// Format table from results
				function formatTable(columns, rows) {
					if (!rows || rows.length === 0) return '';
					
					let html = '<div class="overflow-x-auto mt-2"><table class="min-w-full text-sm border-collapse">';
					html += '<thead><tr class="border-b border-border">';
					columns.forEach(col => {
						html += '<th class="text-left py-2 px-3 text-muted-foreground font-medium">' + escapeHtml(String(col)) + '</th>';
					});
					html += '</tr></thead><tbody>';
					rows.forEach(row => {
						html += '<tr class="border-b border-border/50">';
						row.forEach((cell, idx) => {
							const columnName = columns[idx] || '';
							const formatted = formatCellValue(cell, columnName);
							html += '<td class="py-2 px-3 text-foreground">' + escapeHtml(formatted) + '</td>';
						});
						html += '</tr>';
					});
					html += '</tbody></table></div>';
					return html;
				}

				// Load and display suggestions
				function loadSuggestions() {
					fetch('/chat/suggestions')
						.then(response => response.json())
						.then(data => {
							if (data.suggestions && data.suggestions.length > 0) {
								displaySuggestions(data.suggestions);
							}
						})
						.catch(err => {
							console.error('Failed to load suggestions:', err);
						});
				}
				
				// Display suggestions as chips
				function displaySuggestions(suggestions) {
					const container = document.getElementById('chat-suggestions');
					if (!container) return;
					
					container.innerHTML = '';
					suggestions.forEach(suggestion => {
						const chip = document.createElement('button');
						chip.type = 'button';
						chip.className = 'px-3 py-1.5 text-sm rounded-lg bg-secondary text-secondary-foreground hover:bg-accent border border-border hover:border-ring transition-colors';
						chip.textContent = suggestion.text;
						chip.addEventListener('click', function() {
							chatInput.value = suggestion.text;
							chatInput.focus();
							// Hide suggestions when user selects one
							container.style.display = 'none';
						});
						container.appendChild(chip);
					});
				}
				
				// Hide suggestions when user starts typing
				chatInput.addEventListener('input', function() {
					const container = document.getElementById('chat-suggestions');
					if (container && chatInput.value.trim().length > 0) {
						container.style.display = 'none';
					} else if (container && chatInput.value.trim().length === 0) {
						container.style.display = 'flex';
					}
				});

				// Handle send button click
				chatSend.addEventListener('click', sendMessage);
				
				// Handle Enter key (but allow Shift+Enter for new lines)
				chatInput.addEventListener('keydown', function(e) {
					if (e.key === 'Enter' && !e.shiftKey) {
						e.preventDefault();
						sendMessage();
					}
				});
				
				// Load suggestions on page load
				loadSuggestions();

				// Send message function
				function sendMessage() {
					const question = chatInput.value.trim();
					if (!question) return;

					// Hide suggestions
					const suggestionsContainer = document.getElementById('chat-suggestions');
					if (suggestionsContainer) {
						suggestionsContainer.style.display = 'none';
					}

					// Add user message
					addMessage('user', question);
					
					// Create thinking indicator (replaces the old loading indicator)
					createThinkingIndicator();
					
					// Clear input
					chatInput.value = '';
					chatSend.disabled = true;
					
					// Build request body with optional thread_id from hidden input
					const threadIDInput = document.getElementById('chat-thread-id');
					const requestBody = {
						question: question,
						ledger_id: ledgerID,
						voice_mode: voiceMode // Include voice mode if active
					};
					if (threadIDInput && threadIDInput.value) {
						requestBody.thread_id = threadIDInput.value;
					}

					// Send request with SSE
					fetch('/chat/ask', {
						method: 'POST',
						headers: {
							'Content-Type': 'application/json',
							'Accept': 'text/event-stream',
						},
						body: JSON.stringify(requestBody)
					})
					.then(response => {
						if (!response.ok) {
							throw new Error('Request failed');
						}
						
						const reader = response.body.getReader();
						const decoder = new TextDecoder();
						let buffer = '';

						function readStream() {
							reader.read().then(({ done, value }) => {
								if (done) {
									chatSend.disabled = false;
									return;
								}

								const chunk = decoder.decode(value, { stream: true });
								buffer += chunk;
								const lines = buffer.split('\n');
								buffer = lines.pop() || '';

								lines.forEach(line => {
									if (line.startsWith('event: ')) {
										window._currentEvent = line.substring(7);
									} else if (line.startsWith('data: ')) {
										handleSSEEvent(window._currentEvent, line.substring(6));
									}
								});

								readStream();
							}).catch(err => {
								removeThinkingIndicator();
								addMessage('error', 'Connection error. Please try again.');
								chatSend.disabled = false;
							});
						}

						readStream();
					})
					.catch(err => {
						removeThinkingIndicator();
						addMessage('error', 'Failed to send message. Please try again.');
						chatSend.disabled = false;
					});
				}

				// Handle SSE events from chat stream
				// Events: thinking, thought, summary, results, error, title, done
				function handleSSEEvent(event, data) {
					if (event === 'thinking') {
						// Update thinking indicator text
						if (thinkingIndicatorId) {
							const currentEl = document.getElementById(thinkingIndicatorId + '-current');
							if (currentEl) currentEl.textContent = data;
						}
					} else if (event === 'thought') {
						// Add thought from model's reasoning
						addThought(data);
					} else if (event === 'summary') {
						// Final summary from LLM - complete the thinking indicator with this message
						completeThinkingIndicator(data);
					} else if (event === 'results') {
						// Get response container
						const responseContainer = getResponseContainer();

						// Parse results
						try {
							const result = JSON.parse(data);
							
							// Update thread ID if we got one (new thread created)
							const threadIDInput = document.getElementById('chat-thread-id');
							if (result.thread_id && threadIDInput && !threadIDInput.value) {
								threadIDInput.value = result.thread_id;
								// Update URL
								const url = new URL(window.location);
								url.searchParams.set('t', result.thread_id);
								window.history.replaceState({}, '', url);
								// Title will be sent via SSE 'title' event
							}
							
							// If no summary was sent, complete with default
							completeThinkingIndicator(result.summary || 'Here\'s what I found:');
							
							// Answer is pre-rendered HTML from markdown - use prose styling with proper spacing
							let content = '<div class="prose prose-neutral dark:prose-invert max-w-none text-foreground leading-6 prose-p:my-3 prose-p:leading-6 prose-headings:text-foreground prose-strong:text-foreground prose-li:text-foreground prose-li:my-1 prose-ul:my-2 prose-ol:my-2">' + (result.answer || 'Results:') + '</div>';
							
							// Add chart if visualization is available
							if (result.visualization && result.viz_type && result.viz_type !== 'none' && result.viz_type !== 'table' && result.viz_type !== 'number') {
								const chartId = 'chart-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
								content += '<div class="mt-4 mb-2"><canvas id="' + chartId + '" style="max-height: 400px;"></canvas></div>';
								
								// Render chart after message is added
								setTimeout(function() {
									try {
										const canvas = document.getElementById(chartId);
										if (canvas && window.Chart) {
											const config = JSON.parse(result.visualization);
											new Chart(canvas, config);
										}
									} catch (e) {
										console.error('Failed to render chart:', e);
									}
								}, 100);
							}
							
							// Add table if rows exist and not a number-only visualization
							if (result.rows && result.rows.length > 0 && result.viz_type !== 'number') {
								content += formatTable(result.columns || [], result.rows);
							}
							
							// Add to response container if available, otherwise to chat
							if (responseContainer) {
								responseContainer.classList.remove('hidden');
								addMessage('assistant', content, responseContainer);
							} else {
								addMessage('assistant', content);
							}
							
							// Reset thinking state
							resetThinkingState();
						} catch (e) {
							console.error('Failed to parse results:', e);
							addMessage('error', 'Failed to parse response');
							resetThinkingState();
						}
					} else if (event === 'error') {
						// Remove thinking indicator on error
						removeThinkingIndicator();

						try {
							const error = JSON.parse(data);
							addMessage('error', error.error || 'An error occurred');
						} catch (e) {
							addMessage('error', data || 'An error occurred');
						}
					} else if (event === 'title') {
						// Thread title generated - update the UI
						try {
							const titleData = JSON.parse(data);
							if (titleData.title) {
								// Update the chat title in header
								const titleEl = document.getElementById('chat-title');
								if (titleEl) {
									titleEl.textContent = titleData.title;
								}
								// Update the thread list item if drawer is open
								if (titleData.thread_id) {
									const threadItem = document.querySelector('[hx-get="/chat/threads/' + titleData.thread_id + '/load"] .text-sm.font-medium');
									if (threadItem) {
										threadItem.textContent = titleData.title;
									}
								}
							}
						} catch (e) {
							console.error('Failed to parse title:', e);
						}
					} else if (event === 'done') {
						// Just enable the send button, don't remove thinking
						chatSend.disabled = false;
					}
				}

				// Initial scroll to bottom
				scrollToBottom();

				// =====================================================
				// Voice Chat Functionality
				// =====================================================
				
				// Voice state variables
				let voiceMode = false;
				let voiceWS = null;
				let captureAudioContext = null; // Separate context for audio capture
				let mediaStream = null;
				let audioProcessor = null; // Script processor for audio
				let audioSource = null; // Media stream source
				let isRecording = false;
				let voiceState = 'idle'; // 'idle', 'listening', 'processing', 'speaking'
				let audioLevelBars = [];
				let silenceTimer = null;
				let audioBuffer = []; // Buffer for end-of-speech detection
				let recentAudioLevels = []; // Rolling window of recent audio levels for silence detection
				let speechDetected = false; // Track if we've detected speech in this session
				let selectedDeviceId = null; // Selected audio input device ID
				
				// Voice UI elements (will be initialized)
				let voiceBtn, voiceStatus, voiceStatusText, voiceStatusIcon, voiceTranscription, voiceAudioLevel, voiceStopBtn;
				
				// Voice toggle button handler
				function handleVoiceToggle() {
					voiceMode = !voiceMode;
					console.log('[Voice] Button clicked, voiceMode:', voiceMode);
					
					// Re-initialize elements if not found (in case DOM changed)
					if (!voiceBtn || !voiceStatus) {
						console.log('[Voice] Re-initializing elements...');
						initVoiceElements();
					}
					
					if (voiceMode) {
						// Update button color immediately
						if (voiceBtn) {
							voiceBtn.classList.add('bg-primary', 'text-primary-foreground');
							voiceBtn.classList.remove('bg-secondary', 'text-secondary-foreground');
							console.log('[Voice] Button color updated to blue');
						}
						
						// Show status immediately (before WebSocket connects)
						if (voiceStatus) {
							voiceStatus.classList.remove('hidden');
							console.log('[Voice] Status shown immediately');
						} else {
							console.error('[Voice] voiceStatus element not found when toggling!');
							alert('Voice status element not found. Please refresh the page.');
							voiceMode = false;
							return;
						}
						if (voiceStatusText) {
							voiceStatusText.textContent = 'Connecting...';
						}
						
						startVoiceMode();
					} else {
						stopVoiceMode();
					}
				}
				
				// Stop button handler (for manual end-of-speech or full stop)
				function handleVoiceStop(event) {
					if (event) {
						event.preventDefault();
						event.stopPropagation();
						event.stopImmediatePropagation(); // Stop all other handlers
					}
					
					console.log('[Voice] ===== STOP BUTTON CLICKED =====');
					console.log('[Voice] isRecording:', isRecording);
					console.log('[Voice] voiceState:', voiceState);
					console.log('[Voice] WS state:', voiceWS ? voiceWS.readyState : 'no WS');
					
					// Don't set isRecording = false here - let stopVoiceMode() handle it
					// This prevents the "Already stopped" issue
					
					// Try to send end_speech if WebSocket exists and is open
					if (voiceWS && voiceWS.readyState === WebSocket.OPEN) {
						try {
							const message = {
								type: 'end_speech',
								timestamp: Date.now()
							};
							voiceWS.send(JSON.stringify(message));
							console.log('[Voice] ✅ Successfully sent end_speech signal from stop button');
							
							// Update UI immediately to show we're processing
							updateVoiceStatus('processing', 'Stopping...');
							
							// Stop voice mode after a short delay to let the message be sent
							// But don't wait too long - stop after 500ms regardless
							setTimeout(function() {
								console.log('[Voice] Calling stopVoiceMode() after delay...');
								stopVoiceMode();
							}, 500);
						} catch (err) {
							console.error('[Voice] ❌ Failed to send end_speech:', err);
							// If send failed, stop immediately
							stopVoiceMode();
						}
					} else {
						console.log('[Voice] ⚠️ WebSocket not ready, stopping immediately');
						// WebSocket not ready, stop immediately
						stopVoiceMode();
					}
					
					console.log('[Voice] ===== STOP INITIATED =====');
				}
				
				// Initialize voice UI elements
				function initVoiceElements() {
					voiceBtn = document.getElementById('chat-voice-btn');
					voiceStatus = document.getElementById('voice-status');
					voiceStatusText = document.getElementById('voice-status-text');
					voiceStatusIcon = document.getElementById('voice-status-icon');
					voiceTranscription = document.getElementById('voice-transcription');
					voiceAudioLevel = document.getElementById('voice-audio-level');
					voiceStopBtn = document.getElementById('voice-stop-btn');
					
					console.log('[Voice] Elements initialized:', {
						voiceBtn: !!voiceBtn,
						voiceStatus: !!voiceStatus,
						voiceStatusText: !!voiceStatusText,
						voiceStatusIcon: !!voiceStatusIcon,
						voiceAudioLevel: !!voiceAudioLevel,
						voiceStopBtn: !!voiceStopBtn
					});
					
					// Set up voice button handler (remove any existing first)
					if (voiceBtn) {
						// Clone and replace to remove all event listeners
						const newBtn = voiceBtn.cloneNode(true);
						voiceBtn.parentNode.replaceChild(newBtn, voiceBtn);
						voiceBtn = newBtn;
						
						voiceBtn.addEventListener('click', function(e) {
							e.preventDefault();
							e.stopPropagation();
							console.log('[Voice] Button clicked!');
							handleVoiceToggle();
						});
						console.log('[Voice] Voice button handler attached');
					} else {
						console.error('[Voice] Voice button not found in DOM!');
					}
					
					// Set up stop button handler - use capture to ensure it fires
					if (voiceStopBtn) {
						// Remove any existing listeners by cloning
						const newStopBtn = voiceStopBtn.cloneNode(true);
						voiceStopBtn.parentNode.replaceChild(newStopBtn, voiceStopBtn);
						voiceStopBtn = newStopBtn;
						
						// Add event listener with capture phase to ensure it fires
						voiceStopBtn.addEventListener('click', handleVoiceStop, true);
						console.log('[Voice] Stop button handler attached');
					} else {
						console.error('[Voice] Stop button not found!');
					}
				}
				
				// Initialize elements when DOM is ready
				if (document.readyState === 'loading') {
					document.addEventListener('DOMContentLoaded', function() {
						console.log('[Voice] DOMContentLoaded - initializing');
						initVoiceElements();
						// Also set up a global click handler as fallback
						setupStopButtonFallback();
					});
				} else {
					// DOM already ready
					console.log('[Voice] DOM already ready - initializing');
					initVoiceElements();
					// Also set up a global click handler as fallback
					setupStopButtonFallback();
				}
				
				// Fallback: Use event delegation on the document to catch stop button clicks
				function setupStopButtonFallback() {
					document.addEventListener('click', function(event) {
						// Check if the clicked element is the stop button or a child of it
						const target = event.target;
						if (target && (target.id === 'voice-stop-btn' || target.closest('#voice-stop-btn'))) {
							console.log('[Voice] Stop button clicked (via fallback handler)');
							handleVoiceStop(event);
						}
					}, true); // Use capture phase
					console.log('[Voice] Fallback stop button handler attached to document');
				}


				// Start voice mode
				async function startVoiceMode() {
					console.log('[Voice] startVoiceMode called');
					
					// Verify elements exist
					if (!voiceStatus) {
						console.error('[Voice] voiceStatus element not found!');
						alert('Voice status element not found. Please refresh the page.');
						voiceMode = false;
						return;
					}
					
					try {
						// Update status to connecting
						if (voiceStatusText) {
							voiceStatusText.textContent = 'Requesting microphone access...';
						}
						
						// Request microphone access first (needed to enumerate devices with labels)
						// Use ideal constraints to prefer real devices
						const tempStream = await navigator.mediaDevices.getUserMedia({ 
							audio: {
								echoCancellation: true,
								noiseSuppression: true,
								autoGainControl: true,
								sampleRate: 16000
							}
						});
						
						// Now enumerate devices (they'll have labels after getUserMedia)
						let devices = [];
						try {
							devices = await navigator.mediaDevices.enumerateDevices();
							const audioInputs = devices.filter(d => d.kind === 'audioinput');
							console.log('[Voice] Available audio input devices:', audioInputs.map(d => ({ id: d.deviceId, label: d.label })));
							
							// Stop the temp stream
							tempStream.getTracks().forEach(track => track.stop());
							
							// Filter out virtual devices if there are real ones
							const realDevices = audioInputs.filter(d => {
								const label = d.label.toLowerCase();
								return label && 
									!label.includes('virtual') && 
									!label.includes('zoom') &&
									!label.includes('blackhole') &&
									!label.includes('soundflower') &&
									!label.includes('loopback');
							});
							
							// Use real device if available, otherwise use first available
							if (realDevices.length > 0 && !selectedDeviceId) {
								selectedDeviceId = realDevices[0].deviceId;
								console.log('[Voice] Auto-selected real device:', realDevices[0].label);
							} else if (audioInputs.length > 0 && !selectedDeviceId) {
								selectedDeviceId = audioInputs[0].deviceId;
								console.log('[Voice] Using first available device:', audioInputs[0].label);
							}
						} catch (err) {
							console.warn('[Voice] Could not enumerate devices:', err);
							// Use the temp stream if enumeration fails
							mediaStream = tempStream;
							// Skip requesting a new stream - we already have tempStream
							// Continue to WebSocket setup with the existing stream
						}
						
						// Only request a new microphone stream if enumeration succeeded
						// (If enumeration failed, we're already using tempStream)
						if (devices.length > 0) {
							// Request microphone access with selected device
							const audioConstraints = {
								echoCancellation: true,
								noiseSuppression: true,
								autoGainControl: true,
								sampleRate: 16000
							};
							
							// Add device ID if we have one and enumeration worked
							let stream;
							if (selectedDeviceId && devices.length > 0) {
								audioConstraints.deviceId = { exact: selectedDeviceId };
								console.log('[Voice] Requesting access to specific device:', selectedDeviceId);
								stream = await navigator.mediaDevices.getUserMedia({ 
									audio: audioConstraints
								});
							} else {
								// Fallback: use ideal constraints (browser will choose)
								console.log('[Voice] Requesting access with ideal constraints (browser will choose device)');
								stream = await navigator.mediaDevices.getUserMedia({ 
									audio: audioConstraints
								});
							}
							// Stop the temp stream before replacing it
							if (tempStream && tempStream !== mediaStream) {
								tempStream.getTracks().forEach(track => track.stop());
								console.log('[Voice] Stopped temporary stream before using new stream');
							}
							mediaStream = stream;
						} else {
							// Enumeration failed, using tempStream - no need to request new stream
							console.log('[Voice] Using temporary stream (enumeration failed)');
							// mediaStream is already set to tempStream in the catch block above
						}
						
						// Verify stream has active audio tracks
						// Use mediaStream which is set either from tempStream (enumeration failed) or new stream (enumeration succeeded)
						if (!mediaStream) {
							throw new Error('No media stream available');
						}
						const audioTracks = mediaStream.getAudioTracks();
						console.log('[Voice] Microphone access granted, tracks:', audioTracks.length);
						
						if (audioTracks.length === 0) {
							throw new Error('No audio tracks in media stream');
						}
						
						// Log track details
						audioTracks.forEach((track, index) => {
							console.log('[Voice] Audio track ' + index + ':', {
								enabled: track.enabled,
								muted: track.muted,
								readyState: track.readyState,
								label: track.label,
								settings: track.getSettings()
							});
							
							// Monitor track state changes
							track.onmute = () => {
								console.warn('[Voice] Audio track ' + index + ' was muted!');
								alert('Microphone was muted. Please unmute it to continue.');
							};
							track.onunmute = () => {
								console.log('[Voice] Audio track ' + index + ' was unmuted');
							};
						});

						// Create audio context for processing (capture)
						captureAudioContext = new (window.AudioContext || window.webkitAudioContext)({
							sampleRate: 16000 // Gemini Live requires 16kHz
						});

						// Connect to voice WebSocket (include thread_id if available)
						const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
						const threadIDInput = document.getElementById('chat-thread-id');
						let wsUrl = wsProtocol + '//' + window.location.host + '/chat/voice?ledger_id=' + ledgerID;
						if (threadIDInput && threadIDInput.value) {
							wsUrl += '&thread_id=' + threadIDInput.value;
						}
						voiceWS = new WebSocket(wsUrl);

						voiceWS.onopen = async function() {
							console.log('[Voice] WebSocket connected');
							
							// Button should already be blue from handleVoiceToggle, but ensure it is
							if (voiceBtn) {
								voiceBtn.classList.add('bg-primary', 'text-primary-foreground');
								voiceBtn.classList.remove('bg-secondary', 'text-secondary-foreground');
								console.log('[Voice] Button confirmed active (blue) state');
							}
							
							// Show "Initializing..." status while setting up audio
							updateVoiceStatus('listening', 'Initializing audio...');
							
							// Set recording flag and state
							isRecording = true;
							voiceState = 'listening';
							
							// Await audio capture setup to ensure it's ready before telling user to speak
							await startAudioCapture();
							
							// Wait for audio processor to actually start processing buffers
							// The audio processor needs a moment to connect and start receiving audio data
							// Wait for 200ms to ensure at least a few buffers have been processed
							await new Promise(resolve => setTimeout(resolve, 200));
							
							// Verify audio processor is actually connected and processing
							if (!audioProcessor || !audioSource) {
								console.error('[Voice] Audio processor not ready after initialization');
								updateVoiceStatus('idle', 'Audio initialization failed');
								return;
							}
							
							// Now that audio is ready, tell user to speak
							updateVoiceStatus('listening', 'Listening... Speak now');
							createAudioLevelIndicator();
							console.log('[Voice] ✅ Audio capture ready - user can now speak');
						};

						voiceWS.onmessage = function(event) {
							const msg = JSON.parse(event.data);
							handleVoiceMessage(msg);
						};

						voiceWS.onerror = function(error) {
							console.error('[Voice] WebSocket error:', error);
							voiceMode = false; // Reset voice mode flag
							stopVoiceMode();
						};

						voiceWS.onclose = function() {
							console.log('[Voice] WebSocket closed');
							voiceMode = false; // Reset voice mode flag
							stopVoiceMode();
						};

					} catch (error) {
						console.error('[Voice] Failed to start voice mode:', error);
						console.error('[Voice] Error details:', {
							name: error.name,
							message: error.message,
							stack: error.stack
						});
						
						// Provide more specific error messages
						let errorMessage = 'Failed to start voice mode. ';
						if (error.name === 'NotAllowedError' || error.name === 'PermissionDeniedError') {
							errorMessage += 'Microphone permission was denied. Please grant permission and try again.';
						} else if (error.name === 'NotFoundError' || error.name === 'DevicesNotFoundError') {
							errorMessage += 'No microphone found. Please connect a microphone and try again.';
						} else if (error.name === 'NotReadableError' || error.name === 'TrackStartError') {
							errorMessage += 'Microphone is already in use by another application. Please close other applications using the microphone.';
						} else if (error.name === 'OverconstrainedError' || error.name === 'ConstraintNotSatisfiedError') {
							errorMessage += 'Microphone does not support required settings. Error: ' + error.message;
						} else {
							errorMessage += 'Error: ' + error.message + '. Please check microphone permissions and try again.';
						}
						
						alert(errorMessage);
						voiceMode = false;
						
						// Clean up any partial state
						if (mediaStream) {
							mediaStream.getTracks().forEach(track => track.stop());
							mediaStream = null;
						}
						if (voiceWS) {
							voiceWS.close();
							voiceWS = null;
						}
						stopVoiceMode();
					}
				}

				// Stop voice mode
				function stopVoiceMode() {
					console.log('[Voice] stopVoiceMode called - stopping all audio processing');
					console.log('[Voice] Current state - isRecording:', isRecording, 'voiceState:', voiceState);
					
					voiceMode = false; // Reset voice mode flag
					
					// Save state BEFORE modifying it
					const wasRecording = isRecording;
					const wasProcessing = voiceState === 'processing';
					
					// Set flags FIRST to prevent any more audio from being sent
					// This must happen before disconnecting to prevent race conditions
					isRecording = false;
					voiceState = 'idle';
					
					// Only skip cleanup if we were already fully stopped AND not processing
					if (!wasRecording && !wasProcessing) {
						console.log('[Voice] Already stopped and not processing, skipping cleanup');
						return;
					}
					
					console.log('[Voice] Proceeding with cleanup (wasRecording:', wasRecording, 'wasProcessing:', wasProcessing);
					
					if (silenceTimer) {
						clearTimeout(silenceTimer);
						silenceTimer = null;
					}
					
					// Reset silence detection state
					recentAudioLevels = [];
					speechDetected = false;
					if (typeof window !== 'undefined') {
						window.peakSpeechRMS = 0; // Reset peak speech level
					}
					
					audioBuffer = []; // Clear audio buffer
					
					// Disconnect audio processor first to stop processing
					if (audioProcessor) {
						try {
							audioProcessor.disconnect();
							audioProcessor.onaudioprocess = null; // Remove handler
							console.log('[Voice] Audio processor disconnected');
						} catch (err) {
							console.error('[Voice] Error disconnecting processor:', err);
						}
						audioProcessor = null;
					}
					
					if (audioSource) {
						try {
							audioSource.disconnect();
							console.log('[Voice] Audio source disconnected');
						} catch (err) {
							console.error('[Voice] Error disconnecting source:', err);
						}
						audioSource = null;
					}
					
					if (mediaStream) {
						mediaStream.getTracks().forEach(track => {
							track.stop();
							console.log('[Voice] Stopped media track:', track.label);
						});
						mediaStream = null;
					}

					if (captureAudioContext) {
						captureAudioContext.close().then(() => {
							console.log('[Voice] Audio context closed');
						}).catch(err => {
							console.error('[Voice] Error closing audio context:', err);
						});
						captureAudioContext = null;
					}

					if (voiceWS) {
						if (voiceWS.readyState === WebSocket.OPEN) {
							voiceWS.close();
						}
						voiceWS = null;
					}

					if (voiceBtn) {
						voiceBtn.classList.remove('bg-primary', 'text-primary-foreground');
						voiceBtn.classList.add('bg-secondary', 'text-secondary-foreground');
					}

					// Hide voice status
					if (voiceStatus) {
						voiceStatus.classList.add('hidden');
					}
					if (voiceStopBtn) {
						voiceStopBtn.classList.add('hidden');
					}
					
					// Clear audio level bars
					clearAudioLevelBars();
				}

				// Update voice status display
				function updateVoiceStatus(state, text) {
					console.log('[Voice] updateVoiceStatus called:', state, text);
					voiceState = state;
					
					if (!voiceStatus) {
						console.error('[Voice] voiceStatus element not found in updateVoiceStatus!');
						return;
					}
					
					// Show status container
					voiceStatus.classList.remove('hidden');
					console.log('[Voice] Status container shown');
					
					if (voiceStatusText) {
						voiceStatusText.textContent = text;
						console.log('[Voice] Status text updated:', text);
					} else {
						console.error('[Voice] voiceStatusText element not found!');
					}
					
					if (voiceStatusIcon) {
						// Update icon animation based on state
						voiceStatusIcon.classList.remove('animate-pulse', 'bg-primary/20', 'bg-primary/40', 'bg-muted/20');
						if (state === 'listening') {
							voiceStatusIcon.classList.add('animate-pulse', 'bg-primary/40');
							console.log('[Voice] Icon set to listening (pulsing)');
						} else if (state === 'processing') {
							voiceStatusIcon.classList.add('bg-primary/20');
						} else if (state === 'speaking') {
							voiceStatusIcon.classList.add('bg-muted/20');
						}
					} else {
						console.error('[Voice] voiceStatusIcon element not found!');
					}
					
					// Always get fresh reference to stop button
					const stopBtn = document.getElementById('voice-stop-btn');
					if (stopBtn) {
						if (state === 'listening') {
							stopBtn.classList.remove('hidden');
							// Ensure button is clickable
							stopBtn.style.pointerEvents = 'auto';
							stopBtn.style.cursor = 'pointer';
							stopBtn.disabled = false;
							// Update global reference
							voiceStopBtn = stopBtn;
							// Re-attach handler to ensure it works
							stopBtn.addEventListener('click', handleVoiceStop, true);
							console.log('[Voice] Stop button shown and handler attached');
						} else {
							stopBtn.classList.add('hidden');
						}
					} else {
						console.error('[Voice] Stop button not found in updateVoiceStatus!');
					}
				}

				// Create audio level indicator bars
				function createAudioLevelIndicator() {
					if (!voiceAudioLevel) {
						console.error('[Voice] voiceAudioLevel element not found!');
						return;
					}
					
					console.log('[Voice] Creating audio level indicator');
					audioLevelBars = [];
					voiceAudioLevel.innerHTML = '';
					
					// Create 5 bars for visual feedback
					for (let i = 0; i < 5; i++) {
						const bar = document.createElement('div');
						bar.className = 'w-1 bg-primary/30 rounded-full transition-all duration-100';
						bar.style.height = '4px';
						bar.style.minHeight = '4px';
						voiceAudioLevel.appendChild(bar);
						audioLevelBars.push(bar);
					}
					console.log('[Voice] Created', audioLevelBars.length, 'audio level bars');
				}

				// Update audio level visualization
				function updateAudioLevel(level) {
					// level is 0-1
					if (!audioLevelBars || audioLevelBars.length === 0) {
						console.warn('[Voice] No audio level bars to update');
						return;
					}
					
					// Ensure level is between 0 and 1
					level = Math.max(0, Math.min(1, level));
					
					// Calculate how many bars should be active
					// Use a smoother distribution - not just ceil, but also show partial activation
					const activeBars = level * audioLevelBars.length;
					
					audioLevelBars.forEach((bar, index) => {
						const activationLevel = Math.max(0, Math.min(1, (activeBars - index)));
						
						if (activationLevel > 0.05) {
							// Bar is active - scale height and opacity based on activation
							const intensity = activationLevel;
							const baseHeight = 4;
							const maxHeight = 24;
							const height = baseHeight + (intensity * (maxHeight - baseHeight));
							
							bar.style.height = height + 'px';
							bar.style.minHeight = height + 'px';
							bar.style.maxHeight = maxHeight + 'px';
							bar.style.opacity = (0.5 + intensity * 0.5).toString();
							bar.style.backgroundColor = ''; // Clear inline color
							bar.classList.add('bg-primary');
							bar.classList.remove('bg-primary/30');
						} else {
							// Bar is inactive
							bar.style.height = '4px';
							bar.style.minHeight = '4px';
							bar.style.maxHeight = '4px';
							bar.style.opacity = '0.3';
							bar.style.backgroundColor = ''; // Clear inline color
							bar.classList.remove('bg-primary');
							bar.classList.add('bg-primary/30');
						}
					});
				}

				// Clear audio level bars
				function clearAudioLevelBars() {
					if (audioLevelBars) {
						audioLevelBars.forEach(bar => {
							bar.style.height = '4px';
							bar.style.opacity = '0.3';
							bar.classList.remove('bg-primary');
							bar.classList.add('bg-primary/30');
						});
					}
				}

				// End speech and send for processing
				// force: if true, send even if isRecording is false (for stop button)
				function endSpeechAndSend(force) {
					// Check if we can send - allow force mode for stop button
					if (!force && (!isRecording || voiceState !== 'listening' || !voiceWS || voiceWS.readyState !== WebSocket.OPEN)) {
						console.log('[Voice] Cannot send end_speech - isRecording:', isRecording, 'state:', voiceState, 'WS ready:', voiceWS ? voiceWS.readyState : 'no WS', 'force:', force);
						return;
					}
					
					// If forcing (from stop button), still check WebSocket is ready
					if (force && (!voiceWS || voiceWS.readyState !== WebSocket.OPEN)) {
						console.log('[Voice] Cannot force send end_speech - WS not ready:', voiceWS ? voiceWS.readyState : 'no WS');
						return;
					}
					
					// Set state to processing immediately - this stops silence detection
					voiceState = 'processing';
					
					// Stop audio processing immediately - we've detected end of speech
					// The grace period is only for backend to finalize, not for us to keep processing
					isRecording = false;
					console.log('[Voice] Stopped audio capture - end of speech detected');
					
					// Clear all audio processing state
					audioBuffer = [];
					recentAudioLevels = [];
					speechDetected = false;
					if (typeof window !== 'undefined') {
						window.peakSpeechRMS = 0;
					}
					
					// Clear silence timer
					if (silenceTimer) {
						clearTimeout(silenceTimer);
						silenceTimer = null;
					}
					
					// Send end-of-speech signal
					const message = {
						type: 'end_speech',
						timestamp: Date.now()
					};
					voiceWS.send(JSON.stringify(message));
					console.log('[Voice] Sent end_speech signal (force:', force, ')');
					
					// Update status
					updateVoiceStatus('processing', 'Processing your question...');
					clearAudioLevelBars();
					
					// Don't clear transcription yet - keep it visible until final transcription is added to chat
				}

				// Start audio capture with resampling
				async function startAudioCapture() {
					if (!captureAudioContext || !mediaStream) return;

					try {
						// Verify media stream is still active
						const audioTracks = mediaStream.getAudioTracks();
						if (audioTracks.length === 0) {
							console.error('[Voice] No audio tracks in media stream');
							alert('No audio tracks available. Please check your microphone.');
							return;
						}
						
						const activeTrack = audioTracks[0];
						if (activeTrack.muted || !activeTrack.enabled || activeTrack.readyState !== 'live') {
							console.error('[Voice] Audio track is not active:', {
								muted: activeTrack.muted,
								enabled: activeTrack.enabled,
								readyState: activeTrack.readyState
							});
							alert('Microphone is muted or not active. Please check your microphone settings.');
							return;
						}
						
						console.log('[Voice] Creating media stream source, audio context sample rate:', captureAudioContext.sampleRate);
						audioSource = captureAudioContext.createMediaStreamSource(mediaStream);
						
						// Target sample rate (16kHz for Gemini Live)
						const targetSampleRate = 16000;
						const sourceSampleRate = captureAudioContext.sampleRate;
						console.log('[Voice] Source sample rate:', sourceSampleRate, 'Target:', targetSampleRate);
						
						// Create a script processor for audio processing
						// Note: createScriptProcessor is deprecated but still widely supported
						// For better performance, consider using AudioWorklet in the future
						const bufferSize = 4096;
						try {
							audioProcessor = captureAudioContext.createScriptProcessor(bufferSize, 1, 1);
						} catch (error) {
							console.error('[Voice] Failed to create script processor:', error);
							alert('Audio processing not supported in this browser');
							return;
						}
						
						// Resampling state
						let resampleBuffer = [];
						const ratio = sourceSampleRate / targetSampleRate;
						
						let audioProcessCount = 0;
						let lastNonZeroLevel = 0;
						
						// Reset silence detection state for new session
						recentAudioLevels = [];
						speechDetected = false;
						window.peakSpeechRMS = 0; // Reset peak speech level for adaptive threshold
						const audioLevelHistorySize = 20; // Track last 20 buffers (~1 second at 50ms per buffer)
						audioProcessor.onaudioprocess = function(e) {
							// CRITICAL: Check isRecording FIRST - if false, do nothing
							if (!isRecording) {
								return; // Stop immediately, don't process anything
							}
							
							audioProcessCount++;
							
							// Always calculate and show audio level if recording (regardless of WebSocket state)
							const inputData = e.inputBuffer.getChannelData(0);
							
							// Check if we're getting any non-zero audio data
							let hasNonZero = false;
							for (let i = 0; i < inputData.length; i++) {
								if (Math.abs(inputData[i]) > 0.0001) {
									hasNonZero = true;
									break;
								}
							}
							
							if (audioProcessCount === 1) {
								console.log('[Voice] Audio processor started, first buffer:', {
									length: inputData.length,
									hasNonZero: hasNonZero,
									firstFew: Array.from(inputData.slice(0, 10)).map(v => v.toFixed(4))
								});
							}
							
							if (audioProcessCount % 100 === 0) {
								console.log('[Voice] Audio processor running, count:', audioProcessCount, 
									'isRecording:', isRecording, 
									'voiceState:', voiceState,
									'hasNonZero:', hasNonZero);
							}
							
							// Calculate audio level for visualization and silence detection
							let sum = 0;
							let sumSquared = 0;
							let max = 0;
							for (let i = 0; i < inputData.length; i++) {
								const abs = Math.abs(inputData[i]);
								sum += abs;
								sumSquared += abs * abs;
								max = Math.max(max, abs);
							}
							const avgLevel = sum / inputData.length;
							const peakLevel = max;
							const rms = Math.sqrt(sumSquared / inputData.length); // True RMS for better silence detection
							
							// Update audio level visualization (always, to show mic is working)
							const scaledLevel = Math.min(1, Math.max(0, (rms * 25) + (peakLevel * 20)));
							updateAudioLevel(scaledLevel);
							
							// Track recent audio levels for silence detection
							recentAudioLevels.push(rms);
							if (recentAudioLevels.length > audioLevelHistorySize) {
								recentAudioLevels.shift(); // Keep only last N levels
							}
							
							// Detect if we've had significant speech (for better silence detection)
							// Very low threshold to detect speech easily
							if (rms > 0.003) { // Very low threshold - almost any sound counts as speech
								if (!speechDetected) {
									console.log('[Voice] 🎤 Speech detected! RMS:', rms.toFixed(4));
								}
								speechDetected = true;
							}
							
							if (audioProcessCount % 50 === 0 && scaledLevel > 0.01) {
								console.log('[Voice] Audio level:', { 
									peakLevel: peakLevel.toFixed(4), 
									avgLevel: avgLevel.toFixed(4), 
									rms: rms.toFixed(4), 
									scaledLevel: scaledLevel.toFixed(4),
									speechDetected: speechDetected,
									recentAvg: recentAudioLevels.length > 0 ? (recentAudioLevels.reduce((a, b) => a + b, 0) / recentAudioLevels.length).toFixed(4) : '0'
								});
							}
							
							// Only process and send audio if recording and WebSocket is ready
							// Check isRecording FIRST to prevent any audio from being sent after stop
							if (!isRecording) {
								return; // Stop immediately if not recording
							}
							
							// Allow processing during grace period (voiceState === 'processing')
							if (!voiceWS || voiceWS.readyState !== WebSocket.OPEN || (voiceState !== 'listening' && voiceState !== 'processing')) {
								return; // Don't send, but continue showing audio levels
							}
							
							// End-of-speech detection: improved algorithm with adaptive threshold
							// Only detect silence if:
							// 1. We've previously detected speech (avoid false positives at start)
							// 2. We're in 'listening' state (don't detect silence during processing/speaking)
							if (speechDetected && voiceState === 'listening') {
								// Calculate average of recent audio levels to smooth out noise
								const recentAvg = recentAudioLevels.length > 0 
									? recentAudioLevels.reduce((a, b) => a + b, 0) / recentAudioLevels.length 
									: 0;
								
								// Use adaptive threshold based on peak speech level
								// Track the peak RMS during speech to set a relative threshold
								if (typeof window.peakSpeechRMS === 'undefined') {
									window.peakSpeechRMS = 0;
								}
								if (rms > window.peakSpeechRMS) {
									window.peakSpeechRMS = rms;
								}
								
								// Use a percentage of peak speech as silence threshold
								// Account for background noise that persists even during silence
								// Use 30% of peak to distinguish between speech and background noise
								const relativeThreshold = window.peakSpeechRMS > 0 ? window.peakSpeechRMS * 0.30 : 0.10;
								// Higher absolute threshold to account for typical background noise
								// Based on logs, silence RMS can be ~0.075, so threshold needs to be higher
								const absoluteThreshold = 0.10; // Raised to handle background noise (must be > typical silence RMS)
								const silenceThreshold = Math.max(relativeThreshold, absoluteThreshold);
								
								// Log silence detection state more frequently (removed - will log after isSilent calculation)
								
								// Check both current RMS and recent average
								// Use a more lenient multiplier for recent average (2.0x) to account for averaging window
								// The recent average might still include some speech from before silence started
								const rmsCheck = rms < silenceThreshold;
								const avgCheck = recentAvg < silenceThreshold * 2.0;
								const isSilent = rmsCheck && avgCheck;
								
								// Log every silence check with explicit results
								if (audioProcessCount % 10 === 0) {
									console.log('[Voice] 🔇 Silence check result:', {
										rms: rms.toFixed(4),
										recentAvg: recentAvg.toFixed(4),
										silenceThreshold: silenceThreshold.toFixed(4),
										rmsCheck: rmsCheck,
										avgCheck: avgCheck,
										isSilent: isSilent,
										timerActive: !!silenceTimer,
										voiceState: voiceState,
										isRecording: isRecording
									});
								}
								
								if (isSilent) {
									// Only set/reset timer if we don't already have one running
									// This prevents constantly resetting the timer on every silent buffer
									if (!silenceTimer) {
										console.log('[Voice] ⏱️ Starting silence timer (1500ms) - RMS:', rms.toFixed(4), 'RecentAvg:', recentAvg.toFixed(4));
										silenceTimer = setTimeout(function() {
											// Check if we're still in a valid state when timer fires
											console.log('[Voice] ⏰ Silence timer fired! Checking state...', {
												voiceState: voiceState,
												isRecording: isRecording,
												audioBufferLength: audioBuffer.length,
												timerStillValid: !!silenceTimer
											});
											
											// Clear the timer reference immediately to prevent multiple firings
											silenceTimer = null;
											
											if (voiceState === 'listening' && isRecording && audioBuffer.length > 0) {
												console.log('[Voice] ✅✅✅ Auto-detected end of speech (silence detected)', {
													rms: rms.toFixed(4),
													recentAvg: recentAvg.toFixed(4),
													threshold: silenceThreshold.toFixed(4),
													audioBufferLength: audioBuffer.length
												});
												endSpeechAndSend(false); // Normal auto-detection, not forced
											} else {
												console.log('[Voice] ❌ Silence detected but not sending - state check failed:', {
													voiceState: voiceState,
													isRecording: isRecording,
													audioBufferLength: audioBuffer.length
												});
											}
										}, 1500); // 1500ms (1.5s) of sustained silence - allows for natural pauses in speech
									}
									// If timer already exists, don't reset it - let it continue counting down
								} else {
									// Clear silence timer when speech detected
									if (silenceTimer) {
										console.log('[Voice] 🗣️ Speech detected (not silent), clearing silence timer - RMS:', rms.toFixed(4), 'RecentAvg:', recentAvg.toFixed(4));
										clearTimeout(silenceTimer);
										silenceTimer = null;
									}
								}
							} else if (audioProcessCount % 50 === 0) {
								console.log('[Voice] Waiting for speech detection before silence detection (RMS:', rms.toFixed(4), ')');
							}
							
							// Add to resample buffer
							for (let i = 0; i < inputData.length; i++) {
								resampleBuffer.push(inputData[i]);
								audioBuffer.push(inputData[i]); // Keep for end-of-speech
							}
							
							// Resample: take every Nth sample (simple decimation)
							const resampledLength = Math.floor(resampleBuffer.length / ratio);
							if (resampledLength > 0) {
								const resampled = new Float32Array(resampledLength);
								for (let i = 0; i < resampledLength; i++) {
									const srcIndex = Math.floor(i * ratio);
									resampled[i] = resampleBuffer[srcIndex];
								}
								
								// Remove processed samples from buffer
								const samplesToRemove = Math.floor(resampledLength * ratio);
								resampleBuffer = resampleBuffer.slice(samplesToRemove);
								
								// Convert Float32Array to Int16Array (16-bit PCM)
								const int16Data = new Int16Array(resampled.length);
								for (let i = 0; i < resampled.length; i++) {
									// Clamp and convert to 16-bit integer
									const s = Math.max(-1, Math.min(1, resampled[i]));
									int16Data[i] = s < 0 ? s * 0x8000 : s * 0x7FFF;
								}

								// Double-check state before sending (prevent race conditions)
								// Allow sending audio if:
								// 1. isRecording is true (audio capture active)
								// 2. voiceState is 'listening' OR 'processing' (allow grace period after end_speech)
								// 3. WebSocket is open
								if (!isRecording || (voiceState !== 'listening' && voiceState !== 'processing') || !voiceWS || voiceWS.readyState !== WebSocket.OPEN) {
									console.log('[Voice] Skipping audio send - state changed:', { isRecording, voiceState, wsReady: voiceWS ? voiceWS.readyState : 'no WS' });
									return;
								}
								
								// Send audio chunk to server (send as base64 for JSON compatibility)
								const uint8Array = new Uint8Array(int16Data.buffer);
								const base64 = btoa(String.fromCharCode.apply(null, uint8Array));
								
								const message = {
									type: 'audio',
									data: base64,
									timestamp: Date.now(),
									chunk_id: Date.now().toString(),
									sample_rate: targetSampleRate
								};
								
								// Final check before actually sending (allow 'processing' state for grace period)
								if (isRecording && (voiceState === 'listening' || voiceState === 'processing') && voiceWS && voiceWS.readyState === WebSocket.OPEN) {
									voiceWS.send(JSON.stringify(message));
								} else {
									console.log('[Voice] Prevented audio send - state changed during processing');
								}
							}
						};

						audioSource.connect(audioProcessor);
						audioProcessor.connect(captureAudioContext.destination);
						console.log('[Voice] Audio processor connected and started');
						
					} catch (error) {
						console.error('[Voice] Audio capture error:', error);
					}
				}

				// Handle voice messages from server
				function handleVoiceMessage(msg) {
					switch (msg.type) {
						case 'text':
							// Display text response (same as regular chat)
							updateVoiceStatus('speaking', 'Speaking response...');
							addMessage('assistant', msg.text);
							
							// Reset state for next utterance
							audioBuffer = []; // Clear buffer
							recentAudioLevels = []; // Clear audio level history
							speechDetected = false; // Reset speech detection
							if (typeof window !== 'undefined') {
								window.peakSpeechRMS = 0; // Reset peak speech level
							}
							
							// Clear any existing silence timer
							if (silenceTimer) {
								clearTimeout(silenceTimer);
								silenceTimer = null;
							}
							
							// Stop voice mode after showing response so user can click again
							setTimeout(function() {
								if (voiceMode) {
									console.log('[Voice] Utterance complete, stopping voice mode');
									stopVoiceMode();
								}
							}, 2000);
							break;
						case 'audio':
							// Play audio response (TTS)
							updateVoiceStatus('speaking', 'Speaking response...');
							playAudioChunk(msg.data);
							break;
						case 'transcription':
							// Show transcribed user input in real-time
							if (voiceTranscription && msg.text) {
								voiceTranscription.textContent = '"' + msg.text + '"';
							}
							
							// If we're in 'processing' state, this is the final transcription
							// Add it as a user message in the chat
							if (voiceState === 'processing' && msg.text) {
								console.log('[Voice] Final transcription received, adding as user message:', msg.text);
								addMessage('user', msg.text);
								// Clear the voice status transcription display since it's now in chat
								if (voiceTranscription) {
									voiceTranscription.textContent = '';
								}
								
								// Ensure audio capture is stopped - transcription is complete
								isRecording = false;
								console.log('[Voice] Transcription complete - audio capture stopped');
							}
							break;
						case 'processing':
							// Server is processing the audio
							updateVoiceStatus('processing', 'Processing your question...');
							break;
						case 'thread_id':
							// Update thread ID if we got a new one (from voice or text)
							const threadIDInput = document.getElementById('chat-thread-id');
							if (threadIDInput) {
								if (!threadIDInput.value) {
									threadIDInput.value = msg.text;
									// Update URL
									const url = new URL(window.location);
									url.searchParams.set('t', msg.text);
									window.history.replaceState({}, '', url);
								}
							}
							break;
						case 'error':
							console.error('[Voice] Error from server:', msg.text);
							// Stop voice mode on error
							stopVoiceMode();
							updateVoiceStatus('idle', 'Error occurred');
							addMessage('error', msg.text);
							break;
						case 'ack':
							// Acknowledgment - server received audio
							break;
					}
				}

				// Play audio chunk (TTS) - queue for smooth playback
				let audioQueue = [];
				let isPlaying = false;
				let playbackAudioContext = null;

				async function playAudioChunk(audioData) {
					// Create separate audio context for playback (different from capture context)
					if (!playbackAudioContext) {
						playbackAudioContext = new (window.AudioContext || window.webkitAudioContext)();
					}

					try {
						// Convert base64 or array data to AudioBuffer
						let arrayBuffer;
						if (typeof audioData === 'string') {
							// Base64 decode
							const binaryString = atob(audioData);
							arrayBuffer = new ArrayBuffer(binaryString.length);
							const bytes = new Uint8Array(arrayBuffer);
							for (let i = 0; i < binaryString.length; i++) {
								bytes[i] = binaryString.charCodeAt(i);
							}
						} else if (audioData instanceof Array) {
							// Convert array to Uint8Array
							arrayBuffer = new Uint8Array(audioData).buffer;
						} else {
							// Assume it's already an ArrayBuffer or TypedArray
							arrayBuffer = audioData.buffer || audioData;
						}

						// If it's raw PCM data (16-bit), we need to convert it to WAV format
						// For now, assume the server sends properly formatted audio
						// In production, you might receive WAV, MP3, or Opus encoded audio
						
						// Try to decode as audio (WAV, MP3, etc.)
						try {
							const audioBuffer = await playbackAudioContext.decodeAudioData(arrayBuffer);
							playAudioBuffer(audioBuffer);
						} catch (decodeError) {
							// If decode fails, might be raw PCM - convert to WAV
							console.warn('[Voice] Direct decode failed, attempting PCM to WAV conversion:', decodeError);
							try {
								// Assume 16-bit PCM at 16kHz (mono) - common format for voice
								const sampleRate = 16000;
								const numChannels = 1;
								const bytesPerSample = 2; // 16-bit = 2 bytes
								const numSamples = arrayBuffer.byteLength / bytesPerSample;
								
								// Create WAV file header
								const wavHeader = new ArrayBuffer(44);
								const view = new DataView(wavHeader);
								
								// RIFF header
								const writeString = (offset, string) => {
									for (let i = 0; i < string.length; i++) {
										view.setUint8(offset + i, string.charCodeAt(i));
									}
								};
								writeString(0, 'RIFF');
								view.setUint32(4, 36 + arrayBuffer.byteLength, true); // File size - 8
								writeString(8, 'WAVE');
								writeString(12, 'fmt ');
								view.setUint32(16, 16, true); // fmt chunk size
								view.setUint16(20, 1, true); // audio format (1 = PCM)
								view.setUint16(22, numChannels, true);
								view.setUint32(24, sampleRate, true);
								view.setUint32(28, sampleRate * numChannels * bytesPerSample, true); // byte rate
								view.setUint16(32, numChannels * bytesPerSample, true); // block align
								view.setUint16(34, 16, true); // bits per sample
								writeString(36, 'data');
								view.setUint32(40, arrayBuffer.byteLength, true);
								
								// Combine header + PCM data
								const wavBuffer = new Uint8Array(wavHeader.byteLength + arrayBuffer.byteLength);
								wavBuffer.set(new Uint8Array(wavHeader), 0);
								wavBuffer.set(new Uint8Array(arrayBuffer), wavHeader.byteLength);
								
								// Try to decode the WAV
								const audioBuffer = await playbackAudioContext.decodeAudioData(wavBuffer.buffer);
								playAudioBuffer(audioBuffer);
								console.log('[Voice] Successfully converted PCM to WAV and decoded');
							} catch (pcmError) {
								console.error('[Voice] PCM to WAV conversion also failed:', pcmError);
								// Could not decode audio - user will hear nothing
							}
						}
						
					} catch (error) {
						console.error('[Voice] Audio playback error:', error);
					}
				}

				// Play AudioBuffer with queue management
				function playAudioBuffer(audioBuffer) {
					audioQueue.push(audioBuffer);
					if (!isPlaying) {
						playNext();
					}
				}

				async function playNext() {
					if (audioQueue.length === 0) {
						isPlaying = false;
						return;
					}

					isPlaying = true;
					const audioBuffer = audioQueue.shift();
					
					const source = playbackAudioContext.createBufferSource();
					source.buffer = audioBuffer;
					source.connect(playbackAudioContext.destination);
					
					source.onended = function() {
						playNext();
					};
					
					source.start();
				}
			})();
		`)),
	)
}
