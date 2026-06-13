package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/extraction"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/orchestrator"
	"github.com/asomervell/probably/internal/storage"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	"github.com/google/uuid"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// StatementsUploadForm renders the upload form page
func (hdl *Handlers) StatementsUploadForm(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	accountID, ok := mustParamUUID(w, r, "accountID", "account ID")
	if !ok {
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Verify account belongs to user's ledger
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if account.LedgerID != ledger.ID {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Get existing statement uploads
	statementStore := models.NewStatementUploadStore(hdl.db.Pool)
	uploads, err := statementStore.GetByAccountID(r.Context(), accountID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Error fetching uploads", "err", err)
		uploads = []*models.StatementUpload{}
	}

	// Check if there are any failed uploads for the clear button
	hasFailed := false
	for _, u := range uploads {
		if u.Status == models.StatementUploadStatusFailed {
			hasFailed = true
			break
		}
	}

	var headerActions []g.Node
	if hasFailed {
		headerActions = append(headerActions, h.Button(
			g.Attr("hx-delete", "/accounts/"+account.ID.String()+"/statements/upload/failed"),
			g.Attr("hx-target", "#uploads-list"),
			g.Attr("hx-swap", "outerHTML"),
			g.Attr("hx-confirm", "Delete all failed uploads?"),
			h.Class("text-xs text-destructive hover:text-destructive/80 px-2 py-1 rounded transition-colors"),
			g.Text("Clear Failed"),
		))
	}

	page := layouts.AppLayout("Upload Statement", user.Email, user.ID.String(),
		shadcn.PageHeader("Upload Statement", account.Name,
			h.A(
				h.Href("/accounts/"+account.ID.String()),
				h.Class("inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2.5 text-sm font-medium transition-colors bg-secondary text-secondary-foreground hover:opacity-90 focus:ring-ring"),
				g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m12 19-7-7 7-7"/><path d="M19 12H5"/></svg>`),
				g.Text("Back to Account"),
			),
		),

		// Combined card with upload form and files list
		shadcn.Card(shadcn.CardProps{},
			shadcn.CardHeaderActions("Upload Bank Statement", "Drag and drop files anywhere on this page, or click to select", headerActions...),
			shadcn.CardContentFull(
				// Drop zone
				h.Div(
					h.ID("drop-zone"),
					h.Class("flex flex-col items-center justify-center w-full min-h-48 border-2 border-dashed border-border rounded-lg cursor-pointer transition-all duration-200 hover:border-primary hover:bg-primary/5"),
					h.Div(
						h.ID("drop-content"),
						h.Class("flex flex-col items-center justify-center pt-5 pb-6 pointer-events-none"),
						layouts.IconUpload(),
						h.P(
							h.ID("drop-text"),
							h.Class("mt-2 text-sm text-muted-foreground"),
							h.Span(h.Class("font-medium text-foreground"), g.Text("Click to upload")),
							g.Text(" or drag and drop"),
						),
						h.P(
							h.ID("drop-hint"),
							h.Class("text-xs text-muted-foreground mt-1"),
							g.Text(fmt.Sprintf("PDF, PNG, JPG (max %d MB per file)", hdl.cfg.StatementMaxFileSizeMB)),
						),
					),
					h.Input(
						h.Type("file"),
						h.Name("statement"),
						h.ID("statement-input"),
						h.Multiple(),
						h.Accept(".pdf,.png,.jpg,.jpeg"),
						h.Class("hidden"),
					),
				),

				// Uploads list (always present, hidden if empty)
				h.Div(
					h.ID("uploads-list"),
					h.Class(func() string {
						if len(uploads) == 0 {
							return "divide-y divide-border hidden"
						}
						return "divide-y divide-border"
					}()),
					g.Group(g.Map(uploads, func(upload *models.StatementUpload) g.Node {
						return hdl.renderStatementUpload(upload, account.ID)
					})),
				),
			),
		),

		// Upload script - handles drag/drop and immediate uploads
		g.Raw(fmt.Sprintf(`<script>
(function() {
	const accountID = '%s';
	const uploadUrl = '/accounts/' + accountID + '/statements/upload';
	const maxSize = %d * 1024 * 1024;
	const allowedTypes = ['application/pdf', 'image/png', 'image/jpeg', 'image/jpg'];
	const uploadsList = document.getElementById('uploads-list');
	const uploadsContainer = document.getElementById('uploads-container');
	const dropZone = document.getElementById('drop-zone');
	const fileInput = document.getElementById('statement-input');
	const dropText = document.getElementById('drop-text');
	const dropHint = document.getElementById('drop-hint');
	
	// Make entire page a drop zone
	document.addEventListener('dragover', (e) => {
		e.preventDefault();
		if (!dropZone.contains(e.target)) {
			dropZone.style.borderColor = 'var(--ring)';
			dropZone.style.backgroundColor = 'var(--ring)';
		}
	});
	
	document.addEventListener('dragleave', (e) => {
		e.preventDefault();
		if (!dropZone.contains(e.relatedTarget)) {
			dropZone.style.borderColor = '';
			dropZone.style.backgroundColor = '';
		}
	});
	
	document.addEventListener('drop', (e) => {
		e.preventDefault();
		dropZone.style.borderColor = '';
		dropZone.style.backgroundColor = '';
		
		if (e.dataTransfer.files.length > 0) {
			handleFiles(Array.from(e.dataTransfer.files));
		}
	});
	
	dropZone.addEventListener('click', () => fileInput.click());
	
	dropZone.addEventListener('dragover', (e) => {
		e.preventDefault();
		dropZone.style.borderColor = '#6366f1';
		dropZone.style.backgroundColor = 'rgba(99, 102, 241, 0.1)';
	});
	
	dropZone.addEventListener('dragleave', (e) => {
		e.preventDefault();
		dropZone.style.borderColor = '';
		dropZone.style.backgroundColor = '';
	});
	
	dropZone.addEventListener('drop', (e) => {
		e.preventDefault();
		dropZone.style.borderColor = '';
		dropZone.style.backgroundColor = '';
		
		if (e.dataTransfer.files.length > 0) {
			handleFiles(Array.from(e.dataTransfer.files));
		}
	});
	
	fileInput.addEventListener('change', (e) => {
		if (e.target.files.length > 0) {
			handleFiles(Array.from(e.target.files));
			// Reset input so same file can be selected again
			e.target.value = '';
		}
	});
	
	function handleFiles(files) {
		files.forEach(file => {
			const isValidType = allowedTypes.includes(file.type) || file.name.match(/\\.(pdf|png|jpg|jpeg)$/i);
			const isValidSize = file.size <= maxSize;
			
			if (!isValidType || !isValidSize) {
				showError(file.name, !isValidType ? 'Invalid file type' : 'File too large');
				return;
			}
			
			uploadFile(file);
		});
	}
	
	function showError(filename, message) {
		const errorID = 'error-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
		const errorElement = createUploadElement(errorID, filename, 'failed');
		errorElement.querySelector('p.text-sm.text-destructive').textContent = message;
		ensureUploadsContainer();
		uploadsList.insertBefore(errorElement, uploadsList.firstChild);
	}
	
	function uploadFile(file) {
		// Show uploading state immediately
		const uploadID = 'upload-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
		const uploadElement = createUploadElement(uploadID, file.name, 'uploading');
		ensureUploadsContainer();
		uploadsList.insertBefore(uploadElement, uploadsList.firstChild);
		
		// Upload file
		const formData = new FormData();
		formData.append('statement', file);
		
		fetch(uploadUrl, {
			method: 'POST',
			body: formData,
			headers: {
				'HX-Request': 'true'
			}
		})
		.then(response => {
			if (!response.ok) {
				throw new Error('Upload failed');
			}
			return response.text();
		})
		.then(html => {
			// Parse response to get upload ID
			const parser = new DOMParser();
			const doc = parser.parseFromString(html, 'text/html');
			const uploadDiv = doc.querySelector('[id^="upload-"]');
			if (uploadDiv) {
				const newUploadID = uploadDiv.id.replace('upload-', '');
				// Replace uploading element with actual upload
				const existing = document.getElementById(uploadID);
				if (existing) {
					existing.outerHTML = uploadDiv.outerHTML;
					// Start polling for status updates
					pollUploadStatus(newUploadID);
				} else {
					// Insert new upload if element doesn't exist
					ensureUploadsContainer();
					uploadsList.insertBefore(uploadDiv, uploadsList.firstChild);
					pollUploadStatus(newUploadID);
				}
			} else {
				// If no upload div, update to failed
				updateUploadStatus(uploadID, 'failed', 'Failed to parse upload response');
			}
		})
		.catch(error => {
			console.error('Upload error:', error);
			updateUploadStatus(uploadID, 'failed', error.message);
		});
	}
	
	function createUploadElement(id, filename, status) {
		const div = document.createElement('div');
		div.id = id;
		div.className = 'p-4';
		div.innerHTML = getUploadHTML(id, filename, status, 0, 0, '');
		return div;
	}
	
	function getUploadHTML(id, filename, status, extracted, created, error) {
		const statusColors = {
			'uploading': { color: 'text-primary', bg: 'bg-primary/10', text: 'Uploading...' },
			'uploaded': { color: 'text-muted-foreground', bg: 'bg-muted-foreground/10', text: 'Uploaded' },
			'pending': { color: 'text-muted-foreground', bg: 'bg-muted-foreground/10', text: 'Pending' },
			'processing': { color: 'text-ring', bg: 'bg-ring/10', text: 'Processing' },
			'completed': { color: 'text-chart-2', bg: 'bg-chart-2/10', text: 'Completed' },
			'failed': { color: 'text-destructive', bg: 'bg-destructive/10', text: 'Failed' }
		};
		
		const statusInfo = statusColors[status] || statusColors.pending;
		const errorHtml = error ? '<p class="text-sm text-destructive mt-1">' + escapeHtml(error) + '</p>' : '';
		const deleteBtn = '<button hx-delete="/accounts/' + accountID + '/statements/upload/' + id.replace('upload-', '') + '" hx-target="#' + id + '" hx-swap="delete" hx-confirm="Delete this upload?" class="text-muted-foreground hover:text-destructive p-1 transition-colors" title="Delete">' + getTrashIcon() + '</button>';
		
		return '<div class="flex items-start justify-between gap-4">' +
			'<div class="flex-1">' +
				'<p class="font-medium text-white">' + escapeHtml(filename) + '</p>' +
				'<p class="text-sm text-muted-foreground mt-1">Extracted: ' + extracted + ', Created: ' + created + '</p>' +
				errorHtml +
			'</div>' +
			'<div class="flex items-center gap-2">' +
				'<span class="px-2 py-1 rounded text-xs font-medium ' + statusInfo.bg + ' ' + statusInfo.color + '">' + statusInfo.text + '</span>' +
				deleteBtn +
			'</div>' +
		'</div>';
	}
	
	function updateUploadStatus(id, status, error, extracted, created) {
		const element = document.getElementById(id);
		if (!element) return;
		
		const filename = element.querySelector('p.font-medium')?.textContent || '';
		const currentExtracted = extracted || (element.querySelector('p.text-sm') ? parseInt(element.querySelector('p.text-sm').textContent.match(/Extracted: (\\d+)/)?.[1] || '0') : 0);
		const currentCreated = created || (element.querySelector('p.text-sm') ? parseInt(element.querySelector('p.text-sm').textContent.match(/Created: (\\d+)/)?.[1] || '0') : 0);
		
		element.innerHTML = getUploadHTML(id, filename, status, currentExtracted, currentCreated, error || '');
		// Re-initialize HTMX on the updated content
		if (window.htmx) {
			htmx.process(element);
		}
	}
	
	function pollUploadStatus(uploadID) {
		let pollCount = 0;
		const maxPolls = 300; // Max 10 minutes (300 * 2 seconds)
		
		const pollInterval = setInterval(() => {
			pollCount++;
			if (pollCount > maxPolls) {
				clearInterval(pollInterval);
				return;
			}
			
			fetch('/accounts/' + accountID + '/statements/upload/' + uploadID + '/status', {
				headers: { 'HX-Request': 'true' }
			})
			.then(response => {
				if (!response.ok) {
					return response.text().then(text => {
						throw new Error('Status check failed (' + response.status + '): ' + (text || response.statusText));
					});
				}
				return response.json();
			})
			.then(data => {
				const element = document.getElementById('upload-' + uploadID);
				if (!element) {
					clearInterval(pollInterval);
					return;
				}
				
				// Update status and counts
				const filename = element.querySelector('p.font-medium')?.textContent || '';
				updateUploadStatus('upload-' + uploadID, data.status, data.error_message || '', data.extracted_count || 0, data.created_count || 0);
				
				// Stop polling if completed or failed
				if (data.status === 'completed' || data.status === 'failed') {
					clearInterval(pollInterval);
					// Fetch final HTML to ensure delete button appears for failed
					if (data.status === 'failed') {
						fetch('/accounts/' + accountID + '/statements/upload/' + uploadID + '/status?html=true', {
							headers: { 'HX-Request': 'true' }
						})
						.then(response => response.text())
						.then(html => {
							const parser = new DOMParser();
							const doc = parser.parseFromString(html, 'text/html');
							const newElement = doc.querySelector('#upload-' + uploadID);
							if (newElement) {
								element.outerHTML = newElement.outerHTML;
							}
						})
						.catch(() => {});
					}
				}
			})
			.catch(error => {
				console.error('Status poll error:', error);
				const element = document.getElementById('upload-' + uploadID);
				if (element) {
					const filename = element.querySelector('p.font-medium')?.textContent || '';
					updateUploadStatus('upload-' + uploadID, 'failed', 'Failed to fetch status: ' + error.message, 0, 0);
				}
				clearInterval(pollInterval);
			});
		}, 2000); // Poll every 2 seconds
	}
	
	function ensureUploadsContainer() {
		if (!uploadsList || uploadsList.classList.contains('hidden')) {
			// Show the uploads list if it's hidden
			const list = document.getElementById('uploads-list');
			if (list) {
				list.classList.remove('hidden');
				return list;
			}
		}
		return uploadsList;
	}
	
	function escapeHtml(text) {
		const div = document.createElement('div');
		div.textContent = text;
		return div.innerHTML;
	}
	
	function getTrashIcon() {
		return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>';
	}
})();
</script>`, account.ID.String(), hdl.cfg.StatementMaxFileSizeMB)),
	)

	renderHTML(w, page)
}

// StatementsUploadSingle handles a single file upload (asynchronous)
func (hdl *Handlers) StatementsUploadSingle(w http.ResponseWriter, r *http.Request) {
	accountID, ok := mustParamUUID(w, r, "accountID", "account ID")
	if !ok {
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Verify account belongs to user's ledger
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if account.LedgerID != ledger.ID {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Parse multipart form
	maxSize := int64(hdl.cfg.StatementMaxFileSizeMB) << 20
	if err := r.ParseMultipartForm(maxSize); err != nil {
		http.Error(w, fmt.Sprintf("File too large or invalid form: %v", err), http.StatusBadRequest)
		return
	}

	// Get allowed types
	allowedTypes := strings.Split(hdl.cfg.StatementAllowedTypes, ",")
	for i := range allowedTypes {
		allowedTypes[i] = strings.TrimSpace(allowedTypes[i])
	}

	// Get single file
	files := r.MultipartForm.File["statement"]
	if len(files) == 0 {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	if len(files) > 1 {
		http.Error(w, "Only one file allowed per request", http.StatusBadRequest)
		return
	}

	fileHeader := files[0]

	// Validate file type
	contentType := ""
	if headers := fileHeader.Header; headers != nil {
		if ct := headers.Get("Content-Type"); ct != "" {
			contentType = ct
		}
	}
	if contentType == "" {
		filename := strings.ToLower(fileHeader.Filename)
		if strings.HasSuffix(filename, ".pdf") {
			contentType = "application/pdf"
		} else if strings.HasSuffix(filename, ".png") {
			contentType = "image/png"
		} else if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg") {
			contentType = "image/jpeg"
		}
	}

	allowed := false
	for _, allowedType := range allowedTypes {
		if contentType == allowedType {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, fmt.Sprintf("File type %s not allowed", contentType), http.StatusBadRequest)
		return
	}

	// Validate file size
	if fileHeader.Size > maxSize {
		http.Error(w, fmt.Sprintf("File too large: %d bytes (max: %d MB)", fileHeader.Size, hdl.cfg.StatementMaxFileSizeMB), http.StatusBadRequest)
		return
	}

	// Open and read file
	file, err := fileHeader.Open()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	// Get storage instance
	storageInstance, err := storage.NewStorageFromEnv(r.Context(), hdl.cfg.BaseURL)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to create storage", "err", err)
		http.Error(w, "Storage configuration error", http.StatusInternalServerError)
		return
	}

	// Generate upload ID
	uploadID := uuid.New()

	// Upload to storage
	gcsPath, err := storageInstance.UploadStatement(r.Context(), ledger.ID, account.ID, uploadID, fileHeader.Filename, fileData, contentType)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to upload to storage: %v", err), http.StatusInternalServerError)
		return
	}

	// Create statement upload record with "uploaded" status (not processing yet)
	statementStore := models.NewStatementUploadStore(hdl.db.Pool)
	upload := &models.StatementUpload{
		ID:               uploadID,
		LedgerID:         ledger.ID,
		AccountID:        &account.ID,
		OriginalFilename: fileHeader.Filename,
		GCSPath:          gcsPath,
		FileSizeBytes:    fileHeader.Size,
		ContentType:      contentType,
		Status:           models.StatementUploadStatusPending, // Will be updated to processing in background
	}

	if err := statementStore.Create(r.Context(), upload); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create upload record: %v", err), http.StatusInternalServerError)
		return
	}

	// Process asynchronously in background with a new context (not tied to request)
	// Use context with timeout to prevent runaway processing (30 minutes max)
	// Note: We don't defer cancel() here because we want the context to live for the goroutine
	bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	go func() {
		defer cancel() // Cancel when goroutine finishes
		hdl.processStatementFileAsync(bgCtx, uploadID, account, ledger.ID, storageInstance, allowedTypes)
	}()

	// Return the upload record immediately as HTML fragment
	hdl.renderHTMXSingleUploadResponse(w, r, account, upload)
}

// renderHTMXSingleUploadResponse renders a single upload as HTML fragment
func (hdl *Handlers) renderHTMXSingleUploadResponse(w http.ResponseWriter, r *http.Request, account *models.Account, upload *models.StatementUpload) {
	uploadNode := hdl.renderStatementUpload(upload, account.ID)

	renderHTML(w, uploadNode)
}

// StatementsUploadStatus returns the current status of an upload (for polling)
func (hdl *Handlers) StatementsUploadStatus(w http.ResponseWriter, r *http.Request) {
	accountID, ok := mustParamUUID(w, r, "accountID", "account ID")
	if !ok {
		return
	}

	uploadID, ok := mustParamUUID(w, r, "uploadID", "upload ID")
	if !ok {
		return
	}

	// Verify account belongs to user's ledger
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if account.LedgerID != ledger.ID {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Get upload
	statementStore := models.NewStatementUploadStore(hdl.db.Pool)
	upload, err := statementStore.GetByID(r.Context(), uploadID)
	if err != nil {
		http.Error(w, "Upload not found", http.StatusNotFound)
		return
	}

	// Verify upload belongs to account
	if upload.AccountID == nil || *upload.AccountID != accountID {
		http.Error(w, "Upload not found", http.StatusNotFound)
		return
	}

	// Check if HTML response is requested
	if r.URL.Query().Get("html") == "true" {
		// Return HTML fragment for the upload
		uploadNode := hdl.renderStatementUpload(upload, accountID)
		renderHTML(w, uploadNode)
		return
	}

	// Return JSON status
	respondJSON(w, http.StatusOK, map[string]any{
		"id":              upload.ID,
		"status":          upload.Status,
		"extracted_count": upload.ExtractedCount,
		"created_count":   upload.CreatedCount,
		"error_message":   upload.ErrorMessage,
	})
}

// setUploadStatus calls store.UpdateStatus and logs a warning if the DB write fails.
func setUploadStatus(ctx context.Context, store *models.StatementUploadStore, id uuid.UUID, status models.StatementUploadStatus, extracted, created int, msg string) {
	if err := store.UpdateStatus(ctx, id, status, extracted, created, msg); err != nil {
		slog.WarnContext(ctx, "failed to update upload status", "id", id, "status", status, "err", err)
	}
}

// processStatementFileAsync processes a statement file in the background
func (hdl *Handlers) processStatementFileAsync(
	ctx context.Context,
	uploadID uuid.UUID,
	account *models.Account,
	ledgerID uuid.UUID,
	storageInstance storage.Storage,
	allowedTypes []string,
) {
	statementStore := models.NewStatementUploadStore(hdl.db.Pool)

	// Update status to processing
	setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusProcessing, 0, 0, "")

	// Get LLM router
	// Create orchestrator for statement extraction
	orch, err := orchestrator.NewOrchestrator(hdl.cfg)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create orchestrator", "err", err)
		setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusFailed, 0, 0, fmt.Sprintf("LLM configuration error: %v", err))
		return
	}

	// Create extractor
	extractor, err := extraction.NewStatementExtractor(hdl.cfg, storageInstance, orch)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create extractor", "err", err)
		setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusFailed, 0, 0, fmt.Sprintf("Failed to initialize extractor: %v", err))
		return
	}

	// Get upload record to get file path
	upload, err := statementStore.GetByID(ctx, uploadID)
	if err != nil {
		// Check if error is due to context cancellation/timeout
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			slog.WarnContext(ctx, "Context canceled or timeout for upload", "id", uploadID, "err", err)
			// Don't update status if context was canceled - it might be a timeout
			return
		}
		slog.ErrorContext(ctx, "Failed to get upload", "err", err)
		// Use a background context for the status update since the original context might be canceled
		setUploadStatus(context.Background(), statementStore, uploadID, models.StatementUploadStatusFailed, 0, 0, fmt.Sprintf("Failed to get upload record: %v", err))
		return
	}

	// Verify file exists in storage before attempting extraction
	if exists, err := storageInstance.Exists(ctx, upload.GCSPath); err != nil {
		slog.ErrorContext(ctx, "error checking if file exists in storage", "path", upload.GCSPath, "err", err)
		setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusFailed, 0, 0, fmt.Sprintf("Failed to verify file exists: %v", err))
		return
	} else if !exists {
		slog.WarnContext(ctx, "file does not exist in storage", "path", upload.GCSPath)
		setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusFailed, 0, 0, fmt.Sprintf("File not found in storage (path: %s). The upload may have failed.", upload.GCSPath))
		return
	}

	// Get expense/income accounts
	expenseAccount, err := hdl.accounts.GetOrCreateExpenseAccount(ctx, ledgerID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get expense account", "err", err)
		setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusFailed, 0, 0, fmt.Sprintf("Failed to get expense account: %v", err))
		return
	}

	incomeAccount, err := hdl.accounts.GetOrCreateIncomeAccount(ctx, ledgerID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get income account", "err", err)
		setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusFailed, 0, 0, fmt.Sprintf("Failed to get income account: %v", err))
		return
	}

	// Extract transactions
	extractedTxns, err := extractor.ExtractTransactions(ctx, upload.GCSPath, account.Type)
	if err != nil {
		errorMsg := fmt.Sprintf("Extraction failed: %v", err)
		slog.ErrorContext(ctx, "statement extraction failed", "upload_id", uploadID, "path", upload.GCSPath, "err", err)
		setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusFailed, 0, 0, errorMsg)
		return
	}

	extractedCount := len(extractedTxns)

	validTxns := extraction.ValidateTransactions(ctx, extractedTxns)

	// If all transactions were filtered out, that's okay - just log it
	if len(validTxns) == 0 {
		slog.WarnContext(ctx, "all extracted transactions were filtered out as invalid", "count", extractedCount)
		setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusCompleted, extractedCount, 0, "No valid transactions to import")
		return
	}

	// Check for duplicates (using validated transactions)
	newTxns, err := extraction.CheckDuplicates(ctx, hdl.transactions, account.ID, validTxns)
	if err != nil {
		slog.ErrorContext(ctx, "Error checking duplicates", "err", err)
		newTxns = validTxns
	}

	duplicateCount := len(validTxns) - len(newTxns)

	// Create transactions
	createdCount := 0
	errorCount := 0
	var newTxnIDs []uuid.UUID
	for i, extTxn := range newTxns {
		txnID, err := hdl.createTransactionFromExtracted(ctx, extTxn, account, ledgerID, expenseAccount, incomeAccount)
		if err != nil {
			slog.ErrorContext(ctx, "failed to create extracted transaction", "index", i+1, "total", len(newTxns), "description", extTxn.Description, "date", extTxn.Date.Format("2006-01-02"), "err", err)
			errorCount++
			continue
		}
		createdCount++
		newTxnIDs = append(newTxnIDs, txnID)
		slog.DebugContext(ctx, "created extracted transaction", "index", i+1, "total", len(newTxns), "description", extTxn.Description, "id", txnID, "date", extTxn.Date.Format("2006-01-02"), "amount_cents", extTxn.AmountCents)
	}

	// Queue transactions for AI processing (enrichment + categorization)
	if len(newTxnIDs) > 0 {
		if err := hdl.transactions.QueueForEnrichment(ctx, newTxnIDs); err != nil {
			slog.ErrorContext(ctx, "failed to queue transactions for enrichment", "err", err)
		} else {
			slog.InfoContext(ctx, "queued transactions for AI processing", "count", len(newTxnIDs))
		}
	}

	// Build status message with details
	var statusMsg string
	if duplicateCount > 0 {
		statusMsg = fmt.Sprintf("%d duplicate(s) skipped", duplicateCount)
	}
	if errorCount > 0 {
		if statusMsg != "" {
			statusMsg += fmt.Sprintf(", %d error(s)", errorCount)
		} else {
			statusMsg = fmt.Sprintf("%d error(s) creating transactions", errorCount)
		}
	}
	if createdCount == 0 && duplicateCount == 0 && errorCount == 0 {
		statusMsg = "No valid transactions to import"
	} else if createdCount == 0 {
		if statusMsg == "" {
			statusMsg = "No new transactions created"
		}
	}

	// Update status
	setUploadStatus(ctx, statementStore, uploadID, models.StatementUploadStatusCompleted, extractedCount, createdCount, statusMsg)
}

func (hdl *Handlers) createTransactionFromExtracted(
	ctx context.Context,
	extTxn extraction.ExtractedTransaction,
	account *models.Account,
	ledgerID uuid.UUID,
	expenseAccount *models.Account,
	incomeAccount *models.Account,
) (uuid.UUID, error) {
	// Determine contra account and amount signs based on account type
	var contraAccountID uuid.UUID
	var accountAmountCents, contraAmountCents int64

	if account.Type == models.AccountTypeAsset {
		// For assets: negative = expense (money out), positive = income (money in)
		accountAmountCents = extTxn.AmountCents
		if extTxn.AmountCents < 0 {
			contraAccountID = expenseAccount.ID
			contraAmountCents = -extTxn.AmountCents
		} else {
			contraAccountID = incomeAccount.ID
			contraAmountCents = -extTxn.AmountCents
		}
	} else if account.Type == models.AccountTypeLiability {
		// For liabilities: positive = expense (charge), negative = income (payment)
		accountAmountCents = extTxn.AmountCents
		if extTxn.AmountCents > 0 {
			contraAccountID = expenseAccount.ID
			contraAmountCents = -extTxn.AmountCents
		} else {
			contraAccountID = incomeAccount.ID
			contraAmountCents = -extTxn.AmountCents
		}
	} else {
		return uuid.Nil, fmt.Errorf("unsupported account type for statement upload: %s", account.Type)
	}

	// Create transaction
	txn := &models.Transaction{
		LedgerID:             ledgerID,
		Date:                 extTxn.Date,
		Description:          extTxn.Description,
		CategorizationStatus: models.CategorizationStatusPending,
	}

	// Create entries
	entries := []*models.Entry{
		{AccountID: account.ID, AmountCents: accountAmountCents, Currency: "USD"},
		{AccountID: contraAccountID, AmountCents: contraAmountCents, Currency: "USD"},
	}

	if err := hdl.transactions.CreateWithEntries(ctx, txn, entries); err != nil {
		return uuid.Nil, err
	}

	return txn.ID, nil
}

func buildUploadDetailsText(upload *models.StatementUpload) string {
	msg := fmt.Sprintf("Extracted: %d, Created: %d", upload.ExtractedCount, upload.CreatedCount)
	if upload.ErrorMessage != "" && upload.Status == models.StatementUploadStatusCompleted {
		msg += " • " + upload.ErrorMessage
	}
	return msg
}

func (hdl *Handlers) renderStatementUpload(upload *models.StatementUpload, accountID uuid.UUID) g.Node {
	statusColor := "text-muted-foreground"
	statusBg := "bg-secondary/10"
	statusText := string(upload.Status)

	if upload.Status == models.StatementUploadStatusCompleted {
		statusColor = "text-chart-2"
		statusBg = "bg-chart-2/10"
		statusText = "Completed"
	} else if upload.Status == models.StatementUploadStatusFailed {
		statusColor = "text-destructive"
		statusBg = "bg-destructive/10"
		statusText = "Failed"
	} else if upload.Status == models.StatementUploadStatusProcessing {
		statusColor = "text-ring"
		statusBg = "bg-ring/10"
		statusText = "Processing"
	} else if upload.Status == models.StatementUploadStatusPending {
		statusColor = "text-muted-foreground"
		statusBg = "bg-secondary/10"
		statusText = "Pending"
	}

	return h.Div(
		h.Class("p-4"),
		h.ID("upload-"+upload.ID.String()),
		h.Div(
			h.Class("flex items-start justify-between gap-4"),
			h.Div(
				h.Class("flex-1"),
				h.P(h.Class("font-medium text-foreground"), g.Text(upload.OriginalFilename)),
				h.P(h.Class("text-sm text-muted-foreground mt-1"),
					g.Text(buildUploadDetailsText(upload))),
				g.If(upload.ErrorMessage != "" && upload.Status == models.StatementUploadStatusFailed,
					h.P(h.Class("text-sm text-destructive mt-1"), g.Text(upload.ErrorMessage)),
				),
			),
			h.Div(
				h.Class("flex items-center gap-2"),
				h.Span(
					h.Class(fmt.Sprintf("px-2 py-1 rounded text-xs font-medium %s %s", statusBg, statusColor)),
					g.Text(statusText),
				),
				// Re-queue button for completed uploads (if transactions weren't processed)
				g.If(upload.Status == models.StatementUploadStatusCompleted && upload.CreatedCount > 0,
					h.Button(
						g.Attr("hx-post", "/accounts/"+accountID.String()+"/statements/upload/"+upload.ID.String()+"/requeue"),
						g.Attr("hx-target", "#upload-"+upload.ID.String()),
						g.Attr("hx-swap", "outerHTML"),
						g.Attr("hx-confirm", "Re-queue transactions from this upload for AI processing?"),
						h.Class("text-muted-foreground hover:text-primary p-1 transition-colors"),
						h.Title("Re-queue for processing"),
						g.Raw(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M3 21v-5h5"/></svg>`),
					),
				),
				// Delete button for all uploads
				h.Button(
					g.Attr("hx-delete", "/accounts/"+accountID.String()+"/statements/upload/"+upload.ID.String()),
					g.Attr("hx-target", "#upload-"+upload.ID.String()),
					g.Attr("hx-swap", "delete"),
					g.Attr("hx-confirm", "Delete this upload?"),
					h.Class("text-muted-foreground hover:text-destructive p-1 transition-colors"),
					h.Title("Delete"),
					layouts.IconTrash(),
				),
			),
		),
	)
}

// StatementsUploadDelete deletes a single statement upload
func (hdl *Handlers) StatementsUploadDelete(w http.ResponseWriter, r *http.Request) {
	accountID, ok := mustParamUUID(w, r, "accountID", "account ID")
	if !ok {
		return
	}

	uploadID, ok := mustParamUUID(w, r, "uploadID", "upload ID")
	if !ok {
		return
	}

	// Verify account belongs to user's ledger
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if account.LedgerID != ledger.ID {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Verify upload belongs to account
	statementStore := models.NewStatementUploadStore(hdl.db.Pool)
	upload, err := statementStore.GetByID(r.Context(), uploadID)
	if err != nil {
		http.Error(w, "Upload not found", http.StatusNotFound)
		return
	}
	if upload.AccountID == nil || *upload.AccountID != accountID {
		http.Error(w, "Upload not found", http.StatusNotFound)
		return
	}

	// Delete the upload
	if err := statementStore.Delete(r.Context(), uploadID); err != nil {
		slog.ErrorContext(r.Context(), "Error deleting upload", "err", err)
		http.Error(w, "Failed to delete upload", http.StatusInternalServerError)
		return
	}

	// After deletion, check if there are any uploads left and update the list
	remainingUploads, err := statementStore.GetByAccountID(r.Context(), accountID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Error fetching remaining uploads", "err", err)
		remainingUploads = []*models.StatementUpload{}
	}

	// If no uploads remain, return hidden empty list
	if len(remainingUploads) == 0 {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<div id="uploads-list" class="divide-y divide-border hidden"></div>`))
		return
	}

	// Return 200 with empty body - HTMX with hx-swap="delete" will remove the element
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(""))
}

// StatementsUploadDeleteFailed deletes all failed statement uploads for an account
func (hdl *Handlers) StatementsUploadDeleteFailed(w http.ResponseWriter, r *http.Request) {
	accountID, ok := mustParamUUID(w, r, "accountID", "account ID")
	if !ok {
		return
	}

	// Verify account belongs to user's ledger
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if account.LedgerID != ledger.ID {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Delete all failed uploads
	statementStore := models.NewStatementUploadStore(hdl.db.Pool)
	deletedCount, err := statementStore.DeleteByStatus(r.Context(), accountID, models.StatementUploadStatusFailed)
	if err != nil {
		slog.ErrorContext(r.Context(), "Error deleting failed uploads", "err", err)
		http.Error(w, "Failed to delete uploads", http.StatusInternalServerError)
		return
	}
	slog.InfoContext(r.Context(), "deleted failed uploads", "count", deletedCount)

	// Get remaining uploads to render the updated list
	uploads, err := statementStore.GetByAccountID(r.Context(), accountID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Error fetching remaining uploads", "err", err)
		uploads = []*models.StatementUpload{}
	}

	// Render updated uploads list
	if len(uploads) == 0 {
		// Return hidden empty list if no uploads remain
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<div id="uploads-list" class="divide-y divide-border hidden"></div>`))
		return
	}

	// Render the uploads list
	uploadsListNode := h.Div(
		h.ID("uploads-list"),
		h.Class("divide-y divide-border"),
		g.Group(g.Map(uploads, func(upload *models.StatementUpload) g.Node {
			return hdl.renderStatementUpload(upload, account.ID)
		})),
	)

	var uploadsBuf bytes.Buffer
	if err := uploadsListNode.Render(&uploadsBuf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write(uploadsBuf.Bytes())
}

// StatementsUploadRequeue re-queues transactions from a completed upload for AI processing
func (hdl *Handlers) StatementsUploadRequeue(w http.ResponseWriter, r *http.Request) {
	accountID, ok := mustParamUUID(w, r, "accountID", "account ID")
	if !ok {
		return
	}

	uploadID, ok := mustParamUUID(w, r, "uploadID", "upload ID")
	if !ok {
		return
	}

	// Verify account belongs to user's ledger
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	account, err := hdl.accounts.GetByID(r.Context(), accountID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if account.LedgerID != ledger.ID {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// Get upload
	statementStore := models.NewStatementUploadStore(hdl.db.Pool)
	upload, err := statementStore.GetByID(r.Context(), uploadID)
	if err != nil {
		http.Error(w, "Upload not found", http.StatusNotFound)
		return
	}

	// Verify upload belongs to account
	if upload.AccountID == nil || *upload.AccountID != accountID {
		http.Error(w, "Upload not found", http.StatusNotFound)
		return
	}

	// Find transactions created around the upload's processed time (within 5 minutes)
	// This should catch transactions created from this upload
	var startTime, endTime time.Time
	if upload.ProcessedAt != nil {
		startTime = upload.ProcessedAt.Add(-5 * time.Minute)
		endTime = upload.ProcessedAt.Add(5 * time.Minute)
	} else {
		// Fallback to created_at if processed_at is nil
		startTime = upload.CreatedAt.Add(-5 * time.Minute)
		endTime = upload.CreatedAt.Add(5 * time.Minute)
	}

	// Get transactions for this account created around the upload time
	transactions, _, err := hdl.transactions.List(r.Context(), models.TransactionFilter{
		LedgerID:  ledger.ID,
		AccountID: &accountID,
		Limit:     1000, // Should be enough for any statement
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter to transactions created around the upload time
	var txnIDs []uuid.UUID
	for _, txn := range transactions {
		if txn.CreatedAt.After(startTime) && txn.CreatedAt.Before(endTime) {
			txnIDs = append(txnIDs, txn.ID)
		}
	}

	if len(txnIDs) == 0 {
		// Try a broader time window (1 hour)
		startTime = upload.CreatedAt.Add(-1 * time.Hour)
		endTime = upload.CreatedAt.Add(1 * time.Hour)
		for _, txn := range transactions {
			if txn.CreatedAt.After(startTime) && txn.CreatedAt.Before(endTime) {
				txnIDs = append(txnIDs, txn.ID)
			}
		}
	}

	if len(txnIDs) == 0 {
		http.Error(w, "No transactions found for this upload", http.StatusNotFound)
		return
	}

	// Queue them for processing
	if err := hdl.transactions.QueueForEnrichment(r.Context(), txnIDs); err != nil {
		slog.ErrorContext(r.Context(), "Error queueing transactions", "err", err)
		http.Error(w, "Failed to queue transactions", http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "queued transactions from upload for processing", "count", len(txnIDs), "id", uploadID)

	// Return updated upload HTML
	uploadNode := hdl.renderStatementUpload(upload, accountID)
	renderHTML(w, uploadNode)
}
