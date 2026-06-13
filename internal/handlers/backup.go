package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/asomervell/probably/internal/auth"
	"github.com/asomervell/probably/internal/backup"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/views/layouts"
	"github.com/asomervell/probably/internal/views/shadcn"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// SettingsBackup shows the data backup page
func (hdl *Handlers) SettingsBackup(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check for import success message
	imported := r.URL.Query().Get("imported") == "1"
	importedAccounts := r.URL.Query().Get("accounts")
	importedTransactions := r.URL.Query().Get("transactions")
	importedTags := r.URL.Query().Get("tags")

	// Get some stats for display
	ctx := r.Context()
	accounts, _ := hdl.accounts.GetByLedgerID(ctx, ledger.ID)
	_, total, _ := hdl.transactions.List(ctx, models.TransactionFilter{LedgerID: ledger.ID, Limit: 1})
	tags, _ := hdl.tags.GetByLedgerID(ctx, ledger.ID)

	page := layouts.SettingsLayout("Data Backup", user.Email, "backup", user.ID.String(),
		shadcn.PageHeader("Data Backup", "Export and import your financial data"),

		h.Div(
			h.Class("space-y-6"),

			// Success message
			g.If(imported,
				shadcn.Alert(shadcn.AlertProps{Variant: shadcn.AlertSuccess},
					shadcn.AlertDescription(g.Text(fmt.Sprintf("✓ Import successful! Imported %s accounts, %s transactions, and %s tags.", importedAccounts, importedTransactions, importedTags))),
				),
			),

			// Export card
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardHeaderActions("Export Data", "Download all your data as a ZIP file"),
				shadcn.CardContentFull(
					h.Div(
						h.Class("space-y-4"),

						// Stats
						h.Div(
							h.Class("flex items-center gap-6 mb-6 text-sm"),
							statItem("Accounts", fmt.Sprintf("%d", len(accounts))),
							statItem("Transactions", fmt.Sprintf("%d", total)),
							statItem("Tags", fmt.Sprintf("%d", len(tags))),
						),

						h.P(h.Class("text-sm text-muted-foreground"),
							g.Text("This will export all your accounts, transactions, tags, categorization rules, and transfer matches. Teller access tokens are NOT included for security - you'll need to reconnect your bank accounts after import."),
						),

						h.A(
							h.Href("/settings/backup/download"),
							shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDefault},
								layouts.IconDownload(),
								g.Text("Download Backup"),
							),
						),
					),
				),
			),

			// Import card
			shadcn.Card(shadcn.CardProps{},
				shadcn.CardHeaderActions("Import Data", "Restore data from a backup file"),
				shadcn.CardContentFull(
					h.Div(
						h.Class("space-y-4"),

						h.P(h.Class("text-sm text-muted-foreground"), g.Text("Importing will replace all existing data in your ledger.")),

						h.Form(
							h.Method("POST"),
							h.Action("/settings/backup/import"),
							h.EncType("multipart/form-data"),
							h.ID("import-form"),
							h.Class("space-y-4"),

							// Drag and drop zone
							h.Div(
								h.ID("drop-zone"),
								h.Class("flex flex-col items-center justify-center w-full h-32 border-2 border-dashed border-border rounded-lg cursor-pointer transition-all duration-200"),
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
									h.P(h.ID("drop-hint"), h.Class("text-xs text-muted-foreground mt-1"), g.Text(".zip backup file")),
								),
								h.Input(
									h.Type("file"),
									h.Name("backup_file"),
									h.ID("backup_file"),
									h.Accept(".zip"),
									h.Required(),
									h.Class("hidden"),
								),
							),

							shadcn.Button(shadcn.ButtonProps{Variant: shadcn.ButtonDefault, Type: "submit"},
								h.ID("submit-btn"),
								h.Disabled(),
								g.Text("Import Backup"),
							),
						),

						// Drag and drop script
						g.Raw(`<script>
(function() {
	const dropZone = document.getElementById('drop-zone');
	const fileInput = document.getElementById('backup_file');
	const dropText = document.getElementById('drop-text');
	const dropHint = document.getElementById('drop-hint');
	const form = document.getElementById('import-form');
	const submitBtn = document.getElementById('submit-btn');

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
		
		const files = e.dataTransfer.files;
		if (files.length > 0 && files[0].name.endsWith('.zip')) {
			fileInput.files = files;
			showFileName(files[0].name);
		}
	});

	fileInput.addEventListener('change', () => {
		if (fileInput.files.length > 0) {
			showFileName(fileInput.files[0].name);
		}
	});

	function showFileName(name) {
		dropText.innerHTML = '<span style="color: #34d399; font-weight: 500;">' + name + '</span>';
		dropHint.textContent = 'Ready to import';
		dropZone.style.borderColor = '#34d399';
	}

	form.addEventListener('submit', (e) => {
		if (!fileInput.files.length) {
			e.preventDefault();
			return;
		}
		if (!confirm('Are you sure you want to import this backup? This will REPLACE all your existing data.')) {
			e.preventDefault();
			return;
		}
		submitBtn.disabled = true;
		submitBtn.innerHTML = '<svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg> Importing...';
	});
})();
</script>`),
					),
				),
			),
		),
	)

	renderHTML(w, page)
}

// SettingsBackupDownload generates and downloads the backup ZIP
func (hdl *Handlers) SettingsBackupDownload(w http.ResponseWriter, r *http.Request) {
	ledger, err := hdl.getCurrentLedger(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	zipData, stats, err := backup.Export(ctx, hdl.db.Pool, ledger.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Export failed: %v", err), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "exporting backup", "accounts", stats.Accounts, "tags", stats.Tags, "transactions", stats.Transactions)

	// Set headers for file download
	filename := fmt.Sprintf("probably-backup-%s.zip", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", zipData.Len()))

	_, _ = io.Copy(w, zipData)
}

// SettingsBackupImport handles the backup file upload and import
func (hdl *Handlers) SettingsBackupImport(w http.ResponseWriter, r *http.Request) {
	user := auth.CurrentUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form (max 100MB)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("backup_file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file into memory (needed for zip.NewReader which requires io.ReaderAt)
	fileData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "importing backup file", "filename", header.Filename, "bytes", len(fileData))

	// Import the data
	reader := bytes.NewReader(fileData)
	ctx := r.Context()
	stats, err := backup.Import(ctx, hdl.db.Pool, user.ID, reader, int64(len(fileData)))
	if err != nil {
		http.Error(w, fmt.Sprintf("Import failed: %v", err), http.StatusInternalServerError)
		return
	}

	slog.InfoContext(r.Context(), "backup imported", "accounts", stats.Accounts, "tags", stats.Tags, "transactions", stats.Transactions, "entries", stats.Entries, "rules", stats.Rules, "transaction_tags", stats.TransactionTags)

	// Redirect back to backup page with success message
	http.Redirect(w, r, fmt.Sprintf("/settings/backup?imported=1&accounts=%d&transactions=%d&tags=%d", stats.Accounts, stats.Transactions, stats.Tags), http.StatusSeeOther)
}

// statItem renders a compact stat for the backup page
func statItem(label, value string) g.Node {
	return h.Div(
		h.Class("flex items-center gap-2"),
		h.Span(h.Class("font-semibold text-foreground"), g.Text(value)),
		h.Span(h.Class("text-muted-foreground"), g.Text(label)),
	)
}
