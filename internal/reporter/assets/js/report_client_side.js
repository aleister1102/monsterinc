/**
 * ReportRenderer Class
 *
 * Handles all client-side logic for the interactive HTML report, including:
 * - Data initialization and state management (filtering, sorting, pagination).
 * - Rendering the main results table and pagination controls.
 * - Binding all user interface events (search, filter, sort, etc.).
 * - Populating and displaying the details modal for each result.
 * - Dynamically rendering secret findings within the details modal.
 */
class ReportRenderer {
    constructor () {
        // Initialize data and state
        this.data = window.reportData || [];
        this.filteredData = [...this.data];
        this.currentPage = 1;
        this.itemsPerPage = 25;
        this.totalItems = this.data.length;
        this.totalPages = Math.ceil(this.totalItems / this.itemsPerPage);
        this.currentSort = { column: null, direction: 'asc' };
        this.filters = {
            global: '',
            hostname: '',
            statusCode: '',
            contentType: '',
            technology: '',
            diffStatus: ''
        };

        // View state for switching between probe results and secret findings
        this.currentView = 'probe'; // 'probe' or 'secrets'

        // Secrets pagination state
        this.secretsCurrentPage = 1;
        this.secretsItemsPerPage = 25;
        this.secretsData = [];
        this.secretsTotalItems = 0;
        this.secretsTotalPages = 1;

        // Start the rendering process
        this.init();
    }

    init() {
        // Initial setup
        this.hideLoading();
        this.showControls();
        this.bindEvents();
        this.render();
        this.setupScrollToTop();
        this.setupViewSwitching();
    }

    hideLoading() {
        document.getElementById('loading').style.display = 'none';
    }

    showControls() {
        document.getElementById('controls').style.display = 'block';
        document.getElementById('resultsContainer').style.display = 'block';
        document.getElementById('paginationContainer').style.display = 'block';
        document.getElementById('paginationContainerBottom').style.display = 'block';
    }

    setupViewSwitching() {
        // Add view switching buttons
        const controlsContainer = document.getElementById('controls');
        const viewSwitchHTML = `
            <div class="row mt-3">
                <div class="col-12">
                    <div class="btn-group" role="group" aria-label="View switching">
                        <button type="button" id="probeViewBtn" class="btn btn-primary active">
                            <i class="fas fa-search me-2"></i>Probe Results
                        </button>
                        <button type="button" id="secretsViewBtn" class="btn btn-outline-primary">
                            <i class="fas fa-user-secret me-2"></i>Secret Findings
                        </button>
                    </div>
                </div>
            </div>
        `;
        controlsContainer.insertAdjacentHTML('beforeend', viewSwitchHTML);

        // Bind view switching events
        document.getElementById('probeViewBtn').addEventListener('click', () => this.switchView('probe'));
        document.getElementById('secretsViewBtn').addEventListener('click', () => this.switchView('secrets'));
    }

    switchView(view) {
        this.currentView = view;

        // Update button states
        const probeBtn = document.getElementById('probeViewBtn');
        const secretsBtn = document.getElementById('secretsViewBtn');

        if (view === 'probe') {
            probeBtn.className = 'btn btn-primary active';
            secretsBtn.className = 'btn btn-outline-primary';
            document.getElementById('resultsContainer').style.display = 'block';

            // Hide secrets container if it exists
            const secretsContainer = document.getElementById('secretsContainer');
            if (secretsContainer) {
                secretsContainer.style.display = 'none';
            }
        } else {
            probeBtn.className = 'btn btn-outline-primary';
            secretsBtn.className = 'btn btn-primary active';
            document.getElementById('resultsContainer').style.display = 'none';

            // Ensure secrets container exists and show it
            this.renderSecretsTable();
            const secretsContainer = document.getElementById('secretsContainer');
            if (secretsContainer) {
                secretsContainer.style.display = 'block';
            }
        }

        this.render();
    }

    renderSecretsTable() {
        let secretsContainer = document.getElementById('secretsContainer');
        if (!secretsContainer) {
            // Create secrets container if it doesn't exist
            const resultsContainer = document.getElementById('resultsContainer');
            if (resultsContainer && resultsContainer.parentNode) {
                secretsContainer = document.createElement('div');
                secretsContainer.id = 'secretsContainer';
                secretsContainer.className = 'table-container';
                secretsContainer.style.display = 'none';
                resultsContainer.parentNode.insertBefore(secretsContainer, resultsContainer.nextSibling);
            } else {
                console.error('Cannot create secrets container: resultsContainer not found');
                return;
            }
        }

        // Collect all secret findings
        const secretFindings = [];
        this.filteredData.forEach(item => {
            // Check both field names for compatibility: SecretFindings (Go struct) and secrets (JSON tag)
            const secrets = item.SecretFindings || item.secrets;
            if (secrets && secrets.length > 0) {
                secrets.forEach(secret => {
                    secretFindings.push({
                        url: item.InputURL,
                        ruleId: secret.RuleID,
                        secretText: secret.SecretText,
                        context: secret.Context || 'N/A'
                    });
                });
            }
        });

        // Update secrets pagination data
        this.secretsData = secretFindings;
        this.secretsTotalItems = secretFindings.length;
        this.secretsTotalPages = Math.ceil(this.secretsTotalItems / this.secretsItemsPerPage);

        if (secretFindings.length === 0) {
            secretsContainer.innerHTML = `
                <div class="text-center py-5">
                    <i class="fas fa-shield-alt fa-3x text-muted mb-3"></i>
                    <h4 class="text-muted">No Secret Findings</h4>
                    <p>No secrets were detected in the scanned URLs.</p>
                </div>
            `;
            return;
        }

        // Apply pagination
        const startIndex = (this.secretsCurrentPage - 1) * this.secretsItemsPerPage;
        const endIndex = startIndex + this.secretsItemsPerPage;
        const pageSecrets = secretFindings.slice(startIndex, endIndex);

        secretsContainer.innerHTML = `
            <div class="table-responsive">
                <table class="table table-hover mb-3" id="secretsTable">
                    <thead>
                        <tr>
                            <th style="width: 30%;"><i class="fas fa-link me-1"></i>URL</th>
                            <th style="width: 12%;"><i class="fas fa-user-secret me-1"></i>Rule ID</th>
                            <th style="width: 35%;"><i class="fas fa-key me-1"></i>Secret Text</th>
                            <th style="width: 13%;"><i class="fas fa-info-circle me-1"></i>Context</th>
                            <th style="width: 10%;"><i class="fas fa-cog me-1"></i>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${pageSecrets.map((finding, index) => `
                            <tr>
                                <td>
                                    <a href="${this.escapeHtml(finding.url)}" target="_blank" title="${this.escapeHtml(finding.url)}">
                                        ${this.escapeHtml(this.truncateUrl(finding.url, 50))}
                                    </a>
                                </td>
                                <td><span class="badge bg-danger">${this.escapeHtml(finding.ruleId)}</span></td>
                                <td>
                                    <div class="secret-content" style="max-width: 400px;" title="${this.escapeHtml(finding.secretText)}">
                                        ${this.formatSecretText(finding.secretText)}
                                    </div>
                                </td>
                                <td class="text-center">
                                    <button class="btn btn-info btn-sm" onclick="reportRenderer.showSecretContext(${startIndex + index})" title="View Context">
                                        <i class="fas fa-eye"></i>
                                    </button>
                                </td>
                                <td class="text-center">
                                    <button class="btn btn-success btn-sm me-1" onclick="reportRenderer.copySecret(${startIndex + index})" title="Copy Secret">
                                        <i class="fas fa-copy"></i>
                                    </button>
                                </td>
                            </tr>
                        `).join('')}
                    </tbody>
                </table>
            </div>
        `;
    }

    bindEvents() {
        // Debounced global search
        let searchTimeout;
        document.getElementById('globalSearchInput').addEventListener('input', (e) => {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => {
                this.filters.global = e.target.value.toLowerCase();
                this.applyFilters();
            }, 300);
        });

        // Event listeners for all filter dropdowns
        document.getElementById('rootURLFilter').addEventListener('change', (e) => { this.filters.hostname = e.target.value; this.applyFilters(); });
        document.getElementById('statusCodeFilter').addEventListener('change', (e) => { this.filters.statusCode = e.target.value; this.applyFilters(); });
        document.getElementById('contentTypeFilter').addEventListener('change', (e) => { this.filters.contentType = e.target.value; this.applyFilters(); });
        document.getElementById('technologyFilter').addEventListener('change', (e) => { this.filters.technology = e.target.value; this.applyFilters(); });
        document.getElementById('diffStatusFilter').addEventListener('change', (e) => { this.filters.diffStatus = e.target.value; this.applyFilters(); });

        // Sync items per page dropdowns
        const syncItemsPerPage = (e) => {
            if (this.currentView === 'probe') {
                this.itemsPerPage = parseInt(e.target.value);
                this.totalPages = Math.ceil(this.totalItems / this.itemsPerPage);
                this.currentPage = 1;
            } else {
                this.secretsItemsPerPage = parseInt(e.target.value);
                this.secretsTotalPages = Math.ceil(this.secretsTotalItems / this.secretsItemsPerPage);
                this.secretsCurrentPage = 1;
            }
            document.getElementById('itemsPerPageSelect').value = e.target.value;
            document.getElementById('itemsPerPageSelectBottom').value = e.target.value;
            this.render();
        };
        document.getElementById('itemsPerPageSelect').addEventListener('change', syncItemsPerPage);
        document.getElementById('itemsPerPageSelectBottom').addEventListener('change', syncItemsPerPage);

        // Table sorting
        document.querySelectorAll('.sortable').forEach(th => {
            th.addEventListener('click', () => {
                this.sortData(th.dataset.colName);
            });
        });
    }

    applyFilters() {
        this.filteredData = this.data.filter(item => {
            if (this.filters.global && ![item.InputURL, item.FinalURL, item.Title, item.ContentType, item.WebServer, item.diff_status, ...(item.Technologies || [])].join(' ').toLowerCase().includes(this.filters.global)) return false;
            if (this.filters.hostname && this.extractHostname(item.InputURL) !== this.filters.hostname) return false;
            if (this.filters.statusCode && item.StatusCode.toString() !== this.filters.statusCode) return false;
            if (this.filters.contentType && (!item.ContentType || !item.ContentType.includes(this.filters.contentType))) return false;
            if (this.filters.technology && !(item.Technologies || []).some(tech => (tech.Name || tech).toLowerCase().includes(this.filters.technology.toLowerCase()))) return false;
            if (this.filters.diffStatus && (item.diff_status || '') !== this.filters.diffStatus) return false;
            return true;
        });

        this.totalItems = this.filteredData.length;
        this.totalPages = Math.ceil(this.totalItems / this.itemsPerPage);
        this.currentPage = 1;
        this.render();
    }

    sortData(column) {
        if (this.currentSort.column === column) {
            this.currentSort.direction = this.currentSort.direction === 'asc' ? 'desc' : 'asc';
        } else {
            this.currentSort.column = column;
            this.currentSort.direction = 'asc';
        }

        this.filteredData.sort((a, b) => {
            let aVal = '';
            let bVal = '';

            // Handle URL column mapping to InputURL
            if (column === 'URL') {
                aVal = a['InputURL'] || '';
                bVal = b['InputURL'] || '';
            } else {
                aVal = a[column] || '';
                bVal = b[column] || '';
            }

            if (this.currentSort.direction === 'asc') {
                return aVal.toString().localeCompare(bVal.toString());
            } else {
                return bVal.toString().localeCompare(aVal.toString());
            }
        });

        this.updateSortIndicators();
        this.render();
    }

    updateSortIndicators() {
        document.querySelectorAll('.sortable').forEach(th => {
            th.classList.remove('sort-asc', 'sort-desc');
            if (th.dataset.colName === this.currentSort.column) {
                th.classList.add(`sort-${this.currentSort.direction}`);
            }
        });
    }

    render() {
        if (this.currentView === 'probe') {
            this.renderTable();
            this.renderPagination();
            this.updateResultsInfo();
        } else {
            this.renderSecretsTable();
            this.renderPagination(); // Use shared pagination
            this.updateSecretsResultsInfo();
        }
    }

    renderTable() {
        const tbody = document.querySelector('#resultsTable tbody');
        if (!tbody) return;

        if (this.filteredData.length === 0) {
            tbody.innerHTML = '<tr><td colspan="8" class="text-center text-muted py-4">No results found</td></tr>';
            document.getElementById('noResults').style.display = 'block';
            return;
        }
        document.getElementById('noResults').style.display = 'none';

        const startIndex = (this.currentPage - 1) * this.itemsPerPage;
        const endIndex = startIndex + this.itemsPerPage;
        const pageItems = this.filteredData.slice(startIndex, endIndex);

        tbody.innerHTML = '';
        pageItems.forEach((item, index) => {
            tbody.appendChild(this.createTableRow(item, startIndex + index));
        });
    }

    createTableRow(item) {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>
                <a href="${this.escapeHtml(item.InputURL)}" target="_blank" title="${this.escapeHtml(item.InputURL)}">
                    ${this.escapeHtml(this.truncateUrl(item.InputURL, 40))}
                </a>
            </td>
            <td>
                <a href="${this.escapeHtml(item.FinalURL)}" target="_blank" title="${this.escapeHtml(item.FinalURL)}">
                    ${this.escapeHtml(this.truncateUrl(item.FinalURL, 40))}
                </a>
            </td>
            <td class="text-center">${this.renderDiffStatus(item.diff_status)}</td>
            <td class="text-center">${this.renderStatusCode(item.StatusCode)}</td>
            <td class="hide-on-mobile">${this.escapeHtml(item.Title || '')}</td>
            <td class="hide-on-mobile">${this.escapeHtml(item.ContentType || '')}</td>
            <td class="hide-on-mobile">${this.renderTechnologies(item.Technologies)}</td>
            <td class="text-center"><button class="btn btn-primary btn-sm details-btn"><i class="fas fa-eye"></i></button></td>
        `;
        tr.querySelector('.details-btn').addEventListener('click', () => this.showDetails(item));
        return tr;
    }

    renderDiffStatus(status) {
        if (!status) {
            return '<span class="badge bg-secondary">Unknown</span>';
        }

        const statusLower = status.toLowerCase();
        switch (statusLower) {
            case 'new':
                return '<span class="diff-status-new">New</span>';
            case 'old':
                return '<span class="diff-status-old">Old</span>';
            case 'existing':
                return '<span class="diff-status-existing">Existing</span>';
            default:
                return `<span class="badge bg-secondary">${this.escapeHtml(status)}</span>`;
        }
    }

    renderStatusCode(statusCode) {
        if (!statusCode) {
            return '<span class="text-muted">N/A</span>';
        }

        const code = parseInt(statusCode);
        let className = '';

        // Use specific status class if defined, otherwise use generic class based on category
        if (code >= 200 && code < 300) {
            className = `status-${code}`;
        } else if (code >= 300 && code < 400) {
            className = `status-${code}`;
        } else if (code >= 400 && code < 500) {
            className = `status-${code}`;
        } else if (code >= 500 && code < 600) {
            className = `status-${code}`;
        } else {
            // For any other codes, use generic styling
            className = 'status-generic';
        }

        return `<span class="${className}">${statusCode}</span>`;
    }

    renderTechnologies(technologies) {
        if (!technologies || technologies.length === 0) {
            return '<span class="text-muted">None</span>';
        }
        return technologies.map(tech =>
            `<span class="tech-tag">${this.escapeHtml(tech.Name || tech)}</span>`
        ).join(' ');
    }

    showDetails(item) {
        if (!item) return;

        const container = document.getElementById('probeDetailsContainer');
        container.innerHTML = `
            <div class="row mb-4"><div class="col-md-12"><div class="card h-100"><div class="card-header bg-primary text-white"><h6 class="mb-0"><i class="fas fa-globe me-2"></i>URL Information</h6></div><div class="card-body"><p><strong>Input URL:</strong> <a href="${this.escapeHtml(item.InputURL)}" target="_blank" title="${this.escapeHtml(item.InputURL)}">${this.escapeHtml(this.truncateUrl(item.InputURL, 80))}</a></p><p><strong>Final URL:</strong> <a href="${this.escapeHtml(item.FinalURL)}" target="_blank" title="${this.escapeHtml(item.FinalURL)}">${this.escapeHtml(this.truncateUrl(item.FinalURL, 80))}</a></p><p><strong>Diff Status:</strong> ${this.renderDiffStatus(item.diff_status)}</p></div></div></div></div>
            <div class="row"><div class="col-md-6"><div class="card h-100"><div class="card-header bg-primary text-white"><h6 class="mb-0"><i class="fas fa-server me-2"></i>Response Details</h6></div><div class="card-body"><p><strong>Status Code:</strong> ${this.renderStatusCode(item.StatusCode)}</p><p><strong>Content Type:</strong> ${this.escapeHtml(item.ContentType || 'N/A')}</p><p><strong>Content Length:</strong> ${item.ContentLength || 'N/A'}</p><p><strong>Title:</strong> ${this.escapeHtml(item.Title || 'N/A')}</p><p><strong>Web Server:</strong> ${this.escapeHtml(item.WebServer || 'N/A')}</p></div></div></div><div class="col-md-6"><div class="card h-100"><div class="card-header bg-primary text-white"><h6 class="mb-0"><i class="fas fa-cogs me-2"></i>Technologies</h6></div><div class="card-body">${this.renderTechnologies(item.Technologies)}</div></div></div></div>
            <div class="row mt-4"><div class="col-md-12"><div class="card"><div class="card-header bg-primary text-white"><h6 class="mb-0"><i class="fas fa-file-code me-2"></i>Response Body</h6></div><div class="card-body"><pre><code class="language-html" style="max-height: 300px; overflow-y: auto;">${this.escapeHtml(item.Body || 'No response body available')}</code></pre></div></div></div></div>
            <div class="row mt-4"><div class="col-md-12"><p><strong>Error:</strong> ${this.escapeHtml(item.Error || 'None')}</p><p><strong>Timestamp:</strong> ${item.Timestamp || 'N/A'}</p></div></div>`;

        const secretsSection = document.getElementById('modalSecretsSection');
        // Check both field names for compatibility: SecretFindings (Go struct) and secrets (JSON tag)
        const secrets = item.SecretFindings || item.secrets;
        if (secrets && secrets.length > 0) {
            secretsSection.innerHTML = `
                <h5 class="mt-4"><i class="fas fa-user-secret me-2"></i>Secret Findings</h5>
                <div class="table-responsive">
                    <table class="table table-sm table-bordered">
                        <thead class="table-secondary"><tr><th>Rule ID</th><th>Secret Snippet</th><th>Actions</th></tr></thead>
                        <tbody>${secrets.map((s, index) => {
                let secretText = this.escapeHtml(s.SecretText);
                // Decode unicode escapes
                secretText = secretText.replace(/\\u([0-9a-fA-F]{4})/g, (match, p1) => String.fromCharCode(parseInt(p1, 16)));
                // Truncate if too long (show first 200 chars)
                const truncatedText = secretText.length > 200 ? secretText.substring(0, 200) + '... [truncated]' : secretText;
                const fullSecretText = this.escapeHtml(s.SecretText);
                return `<tr>
                    <td><span class="badge bg-danger">${this.escapeHtml(s.RuleID)}</span></td>
                    <td><div class="secret-content" title="${fullSecretText}">${truncatedText}</div></td>
                    <td class="text-center">
                        <button class="btn btn-success btn-sm" onclick="reportRenderer.copySecretFromModal('${fullSecretText.replace(/'/g, '&#39;')}')" title="Copy Full Secret">
                            <i class="fas fa-copy"></i>
                        </button>
                    </td>
                </tr>`;
            }).join('')}</tbody>
                    </table>
                </div>`;
        } else {
            secretsSection.innerHTML = '';
        }

        new bootstrap.Modal(document.getElementById('detailsModal')).show();
    }

    showSecretContext(secretIndex) {
        const secret = this.secretsData[secretIndex];
        if (!secret) return;

        // Create modal if it doesn't exist
        let modal = document.getElementById('secretContextModal');
        if (!modal) {
            modal = document.createElement('div');
            modal.id = 'secretContextModal';
            modal.className = 'modal fade';
            modal.innerHTML = `
                <div class="modal-dialog modal-lg">
                    <div class="modal-content">
                        <div class="modal-header">
                            <h5 class="modal-title"><i class="fas fa-info-circle me-2"></i>Secret Context</h5>
                            <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
                        </div>
                        <div class="modal-body">
                            <div id="secretContextContent"></div>
                        </div>
                        <div class="modal-footer">
                            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">
                                <i class="fas fa-times me-1"></i>Close
                            </button>
                        </div>
                    </div>
                </div>
            `;
            document.body.appendChild(modal);
        }

        // Populate modal content
        const content = document.getElementById('secretContextContent');
        content.innerHTML = `
            <div class="row mb-3">
                <div class="col-md-3"><strong>URL:</strong></div>
                <div class="col-md-9">
                    <a href="${this.escapeHtml(secret.url)}" target="_blank" class="text-break" title="${this.escapeHtml(secret.url)}">
                        ${this.escapeHtml(this.truncateUrl(secret.url, 80))}
                    </a>
                </div>
            </div>
            <div class="row mb-3">
                <div class="col-md-3"><strong>Rule ID:</strong></div>
                <div class="col-md-9">
                    <span class="badge bg-danger">${this.escapeHtml(secret.ruleId)}</span>
                </div>
            </div>
            <div class="row mb-3">
                <div class="col-md-3"><strong>Secret Text:</strong></div>
                <div class="col-md-9">
                    <div class="p-2 bg-light border rounded">
                        <code class="text-dark"><span class="secret-highlight">${this.escapeHtml(secret.secretText)}</span></code>
                    </div>
                </div>
            </div>
            <div class="row">
                <div class="col-md-3"><strong>Context:</strong></div>
                <div class="col-md-9">
                    <div class="p-3 bg-light border rounded" style="max-height: 300px; overflow-y: auto;">
                        <pre class="mb-0"><code>${this.highlightSecretInContext(secret.context, secret.secretText)}</code></pre>
                    </div>
                </div>
            </div>
        `;

        new bootstrap.Modal(modal).show();
    }

    extractHostname(url) {
        try { return new URL(url).hostname; } catch (e) { return ''; }
    }

    truncateUrl(url, maxLength = 60) {
        if (!url || url.length <= maxLength) return url;

        const halfLength = Math.floor((maxLength - 3) / 2);
        return url.substring(0, halfLength) + '...' + url.substring(url.length - halfLength);
    }

    escapeHtml(text) {
        if (text === null || typeof text === 'undefined') return '';
        return text.toString().replace(/[&<>"']/g, m => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[m]));
    }

    formatSecretText(secretText) {
        if (!secretText) return '';

        // Clean up whitespace and normalize
        let cleaned = secretText.trim()
            .replace(/\s+/g, ' ')           // Replace multiple whitespace with single space
            .replace(/\n+/g, ' ')           // Replace newlines with space
            .replace(/\t+/g, ' ')           // Replace tabs with space
            .replace(/\r+/g, '');           // Remove carriage returns

        // Truncate if too long
        if (cleaned.length > 100) {
            cleaned = cleaned.substring(0, 100) + '...';
        }

        return this.escapeHtml(cleaned);
    }

    highlightSecretInContext(context, secretText) {
        if (!context || !secretText) return this.escapeHtml(context);

        // Escape HTML first
        let result = this.escapeHtml(context);
        const escapedSecret = this.escapeHtml(secretText);

        // For very long secrets, just highlight the first 50 chars to avoid performance issues
        let searchText = escapedSecret;
        if (escapedSecret.length > 100) {
            searchText = escapedSecret.substring(0, 50);
        }

        // Create regex to find secret text (case sensitive for exact match)
        const escapedSecretForRegex = searchText.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        const regex = new RegExp(`(${escapedSecretForRegex})`, 'g');

        // Replace matches with highlighted version
        result = result.replace(regex, '<span class="secret-highlight">$1</span>');

        // Also highlight common key patterns if not already highlighted
        const keyPatterns = [
            /-----BEGIN [A-Z ]+-----/g,
            /-----END [A-Z ]+-----/g,
            /[A-Za-z0-9+\/]{40,}={0,2}/g // Base64-like patterns
        ];

        keyPatterns.forEach(pattern => {
            result = result.replace(pattern, (match) => {
                // Don't double-highlight if already highlighted
                if (result.includes(`<span class="secret-highlight">${match}</span>`)) {
                    return match;
                }
                return `<span class="secret-highlight">${match}</span>`;
            });
        });

        return result;
    }

    renderPagination() {
        if (this.currentView === 'probe') {
            this.renderPaginationForElement('paginationList');
            this.renderPaginationForElement('paginationListBottom');
        } else {
            this.renderSecretsPaginationForElement('paginationList');
            this.renderSecretsPaginationForElement('paginationListBottom');
        }
    }

    renderPaginationForElement(elementId) {
        const ul = document.getElementById(elementId);
        if (!ul) return;
        ul.innerHTML = '';

        if (this.totalPages <= 1) return;

        const createPageItem = (text, page, isDisabled = false, isActive = false) => {
            const li = document.createElement('li');
            li.className = `page-item ${isDisabled ? 'disabled' : ''} ${isActive ? 'active' : ''}`;
            const a = document.createElement('a');
            a.className = 'page-link';
            a.href = '#';
            a.innerHTML = text;
            a.addEventListener('click', (e) => {
                e.preventDefault();
                if (!isDisabled) {
                    this.currentPage = page;
                    this.render();
                }
            });
            li.appendChild(a);
            return li;
        };

        ul.appendChild(createPageItem('&laquo;', 1, this.currentPage === 1));

        let startPage = Math.max(1, this.currentPage - 2);
        let endPage = Math.min(this.totalPages, this.currentPage + 2);

        if (this.currentPage <= 3) endPage = Math.min(5, this.totalPages);
        if (this.currentPage > this.totalPages - 3) startPage = Math.max(1, this.totalPages - 4);

        if (startPage > 1) ul.appendChild(createPageItem('...', startPage - 1));
        for (let i = startPage; i <= endPage; i++) {
            ul.appendChild(createPageItem(i, i, false, this.currentPage === i));
        }
        if (endPage < this.totalPages) ul.appendChild(createPageItem('...', endPage + 1));

        ul.appendChild(createPageItem('&raquo;', this.totalPages, this.currentPage === this.totalPages));
    }

    renderSecretsPaginationForElement(elementId) {
        const ul = document.getElementById(elementId);
        if (!ul) return;
        ul.innerHTML = '';

        if (this.secretsTotalPages <= 1) return;

        const createPageItem = (text, page, isDisabled = false, isActive = false) => {
            const li = document.createElement('li');
            li.className = `page-item ${isDisabled ? 'disabled' : ''} ${isActive ? 'active' : ''}`;
            const a = document.createElement('a');
            a.className = 'page-link';
            a.href = '#';
            a.innerHTML = text;
            a.addEventListener('click', (e) => {
                e.preventDefault();
                if (!isDisabled) {
                    this.secretsCurrentPage = page;
                    this.render();
                }
            });
            li.appendChild(a);
            return li;
        };

        ul.appendChild(createPageItem('&laquo;', 1, this.secretsCurrentPage === 1));

        let startPage = Math.max(1, this.secretsCurrentPage - 2);
        let endPage = Math.min(this.secretsTotalPages, this.secretsCurrentPage + 2);

        if (this.secretsCurrentPage <= 3) endPage = Math.min(5, this.secretsTotalPages);
        if (this.secretsCurrentPage > this.secretsTotalPages - 3) startPage = Math.max(1, this.secretsTotalPages - 4);

        if (startPage > 1) ul.appendChild(createPageItem('...', startPage - 1));
        for (let i = startPage; i <= endPage; i++) {
            ul.appendChild(createPageItem(i, i, false, this.secretsCurrentPage === i));
        }
        if (endPage < this.secretsTotalPages) ul.appendChild(createPageItem('...', endPage + 1));

        ul.appendChild(createPageItem('&raquo;', this.secretsTotalPages, this.secretsCurrentPage === this.secretsTotalPages));
    }

    updateResultsInfo() {
        const start = this.filteredData.length > 0 ? (this.currentPage - 1) * this.itemsPerPage + 1 : 0;
        const end = Math.min(start + this.itemsPerPage - 1, this.totalItems);

        document.getElementById('showingStart').textContent = start;
        document.getElementById('showingEnd').textContent = end;
        document.getElementById('totalItems').textContent = this.totalItems;
        if (document.getElementById('showingStartBottom')) {
            document.getElementById('showingStartBottom').textContent = start;
            document.getElementById('showingEndBottom').textContent = end;
            document.getElementById('totalItemsBottom').textContent = this.totalItems;
        }
    }

    updateSecretsResultsInfo() {
        const start = this.secretsTotalItems > 0 ? (this.secretsCurrentPage - 1) * this.secretsItemsPerPage + 1 : 0;
        const end = Math.min(start + this.secretsItemsPerPage - 1, this.secretsTotalItems);

        document.getElementById('showingStart').textContent = start;
        document.getElementById('showingEnd').textContent = end;
        document.getElementById('totalItems').textContent = this.secretsTotalItems;
        if (document.getElementById('showingStartBottom')) {
            document.getElementById('showingStartBottom').textContent = start;
            document.getElementById('showingEndBottom').textContent = end;
            document.getElementById('totalItemsBottom').textContent = this.secretsTotalItems;
        }
    }

    setupScrollToTop() {
        const toTopBtn = document.getElementById('toTopBtn');
        window.onscroll = () => {
            toTopBtn.style.display = (document.body.scrollTop > 20 || document.documentElement.scrollTop > 20) ? "block" : "none";
        };
        toTopBtn.addEventListener('click', () => {
            document.body.scrollTop = 0;
            document.documentElement.scrollTop = 0;
        });
    }

    copySecret(secretIndex) {
        const secret = this.secretsData[secretIndex];
        if (!secret) {
            this.showNotification('Secret not found', 'error');
            return;
        }

        // Use the Clipboard API if available
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(secret.secretText).then(() => {
                this.showNotification('Secret copied to clipboard!', 'success');
            }).catch(err => {
                console.error('Failed to copy: ', err);
                this.fallbackCopyTextToClipboard(secret.secretText);
            });
        } else {
            // Fallback for older browsers or non-HTTPS
            this.fallbackCopyTextToClipboard(secret.secretText);
        }
    }

    fallbackCopyTextToClipboard(text) {
        const textArea = document.createElement("textarea");
        textArea.value = text;

        // Avoid scrolling to bottom
        textArea.style.top = "0";
        textArea.style.left = "0";
        textArea.style.position = "fixed";
        textArea.style.opacity = "0";

        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();

        try {
            const successful = document.execCommand('copy');
            if (successful) {
                this.showNotification('Secret copied to clipboard!', 'success');
            } else {
                this.showNotification('Failed to copy secret', 'error');
            }
        } catch (err) {
            console.error('Fallback: Oops, unable to copy', err);
            this.showNotification('Failed to copy secret', 'error');
        }

        document.body.removeChild(textArea);
    }

    copySecretFromModal(secretText) {
        // Decode HTML entities
        const decodedText = secretText.replace(/&#39;/g, "'").replace(/&quot;/g, '"').replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>');

        // Use the Clipboard API if available
        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(decodedText).then(() => {
                this.showNotification('Secret copied to clipboard!', 'success');
            }).catch(err => {
                console.error('Failed to copy: ', err);
                this.fallbackCopyTextToClipboard(decodedText);
            });
        } else {
            // Fallback for older browsers or non-HTTPS
            this.fallbackCopyTextToClipboard(decodedText);
        }
    }

    showNotification(message, type = 'info') {
        // Create notification element if it doesn't exist
        let notification = document.getElementById('notification');
        if (!notification) {
            notification = document.createElement('div');
            notification.id = 'notification';
            notification.style.cssText = `
                position: fixed;
                top: 20px;
                right: 20px;
                padding: 12px 20px;
                border-radius: 4px;
                color: white;
                font-weight: 500;
                z-index: 9999;
                transition: opacity 0.3s ease;
                opacity: 0;
                pointer-events: none;
            `;
            document.body.appendChild(notification);
        }

        // Set message and style based on type
        notification.textContent = message;
        switch (type) {
            case 'success':
                notification.style.backgroundColor = '#28a745';
                break;
            case 'error':
                notification.style.backgroundColor = '#dc3545';
                break;
            case 'warning':
                notification.style.backgroundColor = '#ffc107';
                notification.style.color = '#212529';
                break;
            default:
                notification.style.backgroundColor = '#17a2b8';
        }

        // Show notification
        notification.style.opacity = '1';
        notification.style.pointerEvents = 'auto';

        // Hide after 3 seconds
        setTimeout(() => {
            notification.style.opacity = '0';
            notification.style.pointerEvents = 'none';
        }, 3000);
    }
}

// Initialize the report renderer when the DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    try {
        // Use window.reportData that's embedded in the template
        if (window.reportData) {
            window.reportRenderer = new ReportRenderer();
        } else {
            throw new Error('Report data not found');
        }
    } catch (e) {
        console.error('Failed to parse report data:', e);
        document.getElementById('loading').innerHTML = '<h4>Error: Failed to load report data.</h4>';
    }
});