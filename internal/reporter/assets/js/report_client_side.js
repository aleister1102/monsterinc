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
    constructor() {
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
            this.itemsPerPage = parseInt(e.target.value);
            this.totalPages = Math.ceil(this.totalItems / this.itemsPerPage);
            this.currentPage = 1;
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
            if (this.filters.global && ![item.InputURL, item.FinalURL, item.Title, item.ContentType, item.WebServer, item.URLStatus, ...(item.Technologies || [])].join(' ').toLowerCase().includes(this.filters.global)) return false;
            if (this.filters.hostname && this.extractHostname(item.InputURL) !== this.filters.hostname) return false;
            if (this.filters.statusCode && item.StatusCode.toString() !== this.filters.statusCode) return false;
            if (this.filters.contentType && (!item.ContentType || !item.ContentType.includes(this.filters.contentType))) return false;
            if (this.filters.technology && !(item.Technologies || []).some(tech => (tech.Name || tech).toLowerCase().includes(this.filters.technology.toLowerCase()))) return false;
            if (this.filters.diffStatus && (item.URLStatus || item.diff_status || '') !== this.filters.diffStatus) return false;
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
            let aVal = a[column] || '';
            let bVal = b[column] || '';
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
        this.renderTable();
        this.renderPagination();
        this.updateResultsInfo();
    }

    renderTable() {
        const tbody = document.querySelector('#resultsTable tbody');
        if (!tbody) return;

        if (this.filteredData.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" class="text-center text-muted py-4">No results found</td></tr>';
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
            <td><a href="${this.escapeHtml(item.InputURL)}" target="_blank">${this.escapeHtml(item.InputURL)}</a></td>
            <td class="text-center"><span class="badge bg-info">${this.escapeHtml(item.URLStatus || 'N/A')}</span></td>
            <td class="text-center">${item.StatusCode || 'N/A'}</td>
            <td class="hide-on-mobile">${this.escapeHtml(item.Title || '')}</td>
            <td class="hide-on-mobile">${this.escapeHtml(item.ContentType || '')}</td>
            <td class="hide-on-mobile">${this.renderTechnologies(item.Technologies)}</td>
            <td class="text-center"><button class="btn btn-primary btn-sm details-btn"><i class="fas fa-eye"></i></button></td>
        `;
        tr.querySelector('.details-btn').addEventListener('click', () => this.showDetails(item));
        return tr;
    }

    renderTechnologies(technologies) {
        if (!technologies || technologies.length === 0) return '<span class="text-muted">None</span>';
        return technologies.map(tech => `<span class="badge bg-secondary me-1">${this.escapeHtml(tech.Name || tech)}</span>`).join(' ');
    }

    showDetails(item) {
        if (!item) return;

        const container = document.getElementById('probeDetailsContainer');
        container.innerHTML = `
            <div class="row mb-4"><div class="col-md-12"><div class="card h-100"><div class="card-header bg-primary text-white"><h6 class="mb-0"><i class="fas fa-globe me-2"></i>URL Information</h6></div><div class="card-body"><p><strong>Input URL:</strong> <a href="${this.escapeHtml(item.InputURL)}" target="_blank">${this.escapeHtml(item.InputURL)}</a></p><p><strong>Final URL:</strong> <a href="${this.escapeHtml(item.FinalURL)}" target="_blank">${this.escapeHtml(item.FinalURL)}</a></p></div></div></div></div>
            <div class="row"><div class="col-md-6"><div class="card h-100"><div class="card-header bg-primary text-white"><h6 class="mb-0"><i class="fas fa-server me-2"></i>Response Details</h6></div><div class="card-body"><p><strong>Status Code:</strong> ${item.StatusCode || 'N/A'}</p><p><strong>Content Type:</strong> ${this.escapeHtml(item.ContentType || 'N/A')}</p><p><strong>Content Length:</strong> ${item.ContentLength || 'N/A'}</p><p><strong>Title:</strong> ${this.escapeHtml(item.Title || 'N/A')}</p><p><strong>Web Server:</strong> ${this.escapeHtml(item.WebServer || 'N/A')}</p></div></div></div><div class="col-md-6"><div class="card h-100"><div class="card-header bg-primary text-white"><h6 class="mb-0"><i class="fas fa-cogs me-2"></i>Technologies</h6></div><div class="card-body">${this.renderTechnologies(item.Technologies)}</div></div></div></div>
            <div class="row mt-4"><div class="col-md-12"><div class="card"><div class="card-header bg-primary text-white"><h6 class="mb-0"><i class="fas fa-file-code me-2"></i>Response Body</h6></div><div class="card-body"><pre><code class="language-html" style="max-height: 300px; overflow-y: auto;">${this.escapeHtml(item.Body || 'No response body available')}</code></pre></div></div></div></div>
            <div class="row mt-4"><div class="col-md-12"><p><strong>Error:</strong> ${this.escapeHtml(item.Error || 'None')}</p><p><strong>Timestamp:</strong> ${item.Timestamp || 'N/A'}</p></div></div>`;

        const secretsSection = document.getElementById('modalSecretsSection');
        if (item.SecretFindings && item.SecretFindings.length > 0) {
            secretsSection.innerHTML = `
                <h5 class="mt-4"><i class="fas fa-user-secret me-2"></i>Secret Findings</h5>
                <div class="table-responsive">
                    <table class="table table-sm table-bordered">
                        <thead class="table-secondary"><tr><th>Rule ID</th><th>Secret Snippet</th></tr></thead>
                        <tbody>${item.SecretFindings.map(s => `<tr><td><span class="badge bg-danger">${this.escapeHtml(s.RuleID)}</span></td><td><pre><code>${this.escapeHtml(s.SecretText)}</code></pre></td></tr>`).join('')}</tbody>
                    </table>
                </div>`;
        } else {
            secretsSection.innerHTML = '';
        }

        new bootstrap.Modal(document.getElementById('detailsModal')).show();
    }

    extractHostname(url) {
        try { return new URL(url).hostname; } catch (e) { return ''; }
    }

    escapeHtml(text) {
        if (text === null || typeof text === 'undefined') return '';
        return text.toString().replace(/[&<>"']/g, m => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[m]));
    }

    renderPagination() {
        this.renderPaginationForElement('paginationList');
        this.renderPaginationForElement('paginationListBottom');
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

    updateResultsInfo() {
        const start = this.filteredData.length > 0 ? (this.currentPage - 1) * this.itemsPerPage + 1 : 0;
        const end = Math.min(start + this.itemsPerPage - 1, this.totalItems);
        
        document.getElementById('showingStart').textContent = start;
        document.getElementById('showingEnd').textContent = end;
        document.getElementById('totalItems').textContent = this.totalItems;
        document.getElementById('showingStartBottom').textContent = start;
        document.getElementById('showingEndBottom').textContent = end;
        document.getElementById('totalItemsBottom').textContent = this.totalItems;
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
}

// Initialize the report renderer when the DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    try {
        // Use window.reportData that's embedded in the template
        if (window.reportData) {
            new ReportRenderer();
        } else {
            throw new Error('Report data not found');
        }
    } catch (e) {
        console.error('Failed to parse report data:', e);
        document.getElementById('loading').innerHTML = '<h4>Error: Failed to load report data.</h4>';
    }
});