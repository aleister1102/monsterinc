/**
 * Client-side JavaScript for the MonsterInc HTML Report
 * Provides interactive features:
 * - Responsive DataTables for probe results filtering and pagination.
 * - Modal functionality for viewing detailed probe result information.
 * - Technology filtering and visual feedback.
 * - Responsive design adaptations.
 */

class ReportManager {
    constructor (probeResultsData) {
        this.probeResults = probeResultsData || [];
        this.currentPage = 1;
        this.itemsPerPage = 25;
        this.totalItems = this.probeResults.length;
        this.totalPages = Math.ceil(this.totalItems / this.itemsPerPage);
        this.table = null; // DataTable instance

        // Filter state
        this.globalFilter = '';
        this.statusFilter = '';
        this.techFilter = '';
        this.hostnameFilter = '';
        this.rootTargetFilter = '';

        // View state for switching between probe results
        this.currentView = 'probe';

        this.initializeTable();
    }

    initializeTable() {
        if ($.fn.DataTable) {
            this.setupDataTable();
        } else {
            console.warn('DataTables is not available. Falling back to manual table management.');
            this.setupManualPagination();
        }
    }

    switchView(viewType) {
        this.currentView = viewType;

        // Update button states
        const probeBtn = document.getElementById('probeViewBtn');

        if (viewType === 'probe') {
            probeBtn.className = 'btn btn-primary active';

            // Hide any other containers and show probe results
            const resultsContainer = document.getElementById('resultsContainer');
            if (resultsContainer) {
                resultsContainer.style.display = 'block';
            }
        }
    }

    renderTable() {
        const startIndex = (this.currentPage - 1) * this.itemsPerPage;
        const endIndex = startIndex + this.itemsPerPage;
        const pageData = this.probeResults.slice(startIndex, endIndex);

        const tableBody = document.getElementById('resultsTableBody');
        if (!tableBody) {
            console.error('Table body element not found');
            return;
        }

        tableBody.innerHTML = '';

        pageData.forEach((item, index) => {
            if (!item) return;

            const row = this.createTableRow(item, startIndex + index);
            tableBody.appendChild(row);
        });

        this.updatePaginationControls();
    }

    createTableRow(item, index) {
        const row = document.createElement('tr');
        row.onclick = () => this.openDetailModal(item);
        row.style.cursor = 'pointer';

        // Status badge
        const statusClass = this.getStatusBadgeClass(item.StatusCode);
        const statusBadge = `<span class="badge ${statusClass}">${item.StatusCode || 'N/A'}</span>`;

        // Technologies (limit display to avoid wide columns)
        const technologies = item.Technologies || [];
        const techDisplay = technologies.length > 0
            ? technologies.slice(0, 3).join(', ') + (technologies.length > 3 ? '...' : '')
            : 'N/A';

        // Content length formatting
        const contentLength = item.ContentLength ? this.formatBytes(item.ContentLength) : 'N/A';

        // URL status for diff (if available)
        const urlStatus = item.URLStatus || '';
        const diffBadge = urlStatus ? `<span class="badge badge-info">${urlStatus}</span>` : '';

        row.innerHTML = `
            <td class="text-truncate" style="max-width: 300px;" title="${this.escapeHtml(item.InputURL || '')}">${this.escapeHtml(item.InputURL || '')}</td>
            <td class="text-truncate" style="max-width: 200px;" title="${this.escapeHtml(item.FinalURL || '')}">${this.escapeHtml(item.FinalURL || '')}</td>
            <td>${statusBadge}</td>
            <td class="text-truncate" style="max-width: 200px;" title="${this.escapeHtml(item.Title || '')}">${this.escapeHtml(item.Title || '')}</td>
            <td class="text-truncate" style="max-width: 150px;" title="${this.escapeHtml(techDisplay)}">${this.escapeHtml(techDisplay)}</td>
            <td class="text-truncate" style="max-width: 120px;" title="${this.escapeHtml(item.WebServer || '')}">${this.escapeHtml(item.WebServer || '')}</td>
            <td class="text-truncate" style="max-width: 120px;" title="${this.escapeHtml(item.ContentType || '')}">${this.escapeHtml(item.ContentType || '')}</td>
            <td>${contentLength}</td>
            <td>${diffBadge}</td>
        `;

        return row;
    }

    openDetailModal(item) {
        // Populate modal with probe result details
        document.getElementById('modalInputURL').textContent = item.InputURL || 'N/A';
        document.getElementById('modalFinalURL').textContent = item.FinalURL || 'N/A';
        document.getElementById('modalMethod').textContent = item.Method || 'N/A';
        document.getElementById('modalStatusCode').textContent = item.StatusCode || 'N/A';
        document.getElementById('modalContentLength').textContent = this.formatBytes(item.ContentLength) || 'N/A';
        document.getElementById('modalContentType').textContent = item.ContentType || 'N/A';
        document.getElementById('modalTitle').textContent = item.Title || 'N/A';
        document.getElementById('modalWebServer').textContent = item.WebServer || 'N/A';
        document.getElementById('modalDuration').textContent = item.Duration ? `${item.Duration.toFixed(2)}s` : 'N/A';
        document.getElementById('modalTimestamp').textContent = item.Timestamp || 'N/A';
        document.getElementById('modalError').textContent = item.Error || 'None';

        // Technologies
        const technologies = item.Technologies || [];
        const techList = document.getElementById('modalTechnologies');
        techList.innerHTML = '';
        if (technologies.length > 0) {
            technologies.forEach(tech => {
                const listItem = document.createElement('li');
                listItem.textContent = tech;
                listItem.className = 'list-group-item';
                techList.appendChild(listItem);
            });
        } else {
            const listItem = document.createElement('li');
            listItem.textContent = 'No technologies detected';
            listItem.className = 'list-group-item text-muted';
            techList.appendChild(listItem);
        }

        // IPs
        const ips = item.IPs || [];
        document.getElementById('modalIPs').textContent = ips.length > 0 ? ips.join(', ') : 'N/A';

        // CNAMEs
        const cnames = item.CNAMEs || [];
        document.getElementById('modalCNAMEs').textContent = cnames.length > 0 ? cnames.join(', ') : 'N/A';

        // ASN
        document.getElementById('modalASN').textContent = item.ASN || 'N/A';
        document.getElementById('modalASNOrg').textContent = item.ASNOrg || 'N/A';

        // TLS Information
        document.getElementById('modalTLSVersion').textContent = item.TLSVersion || 'N/A';
        document.getElementById('modalTLSCipher').textContent = item.TLSCipher || 'N/A';
        document.getElementById('modalTLSCertIssuer').textContent = item.TLSCertIssuer || 'N/A';
        document.getElementById('modalTLSCertExpiry').textContent = item.TLSCertExpiry || 'N/A';

        // Headers
        const headers = item.Headers || {};
        const headersList = document.getElementById('modalHeaders');
        headersList.innerHTML = '';
        const headerEntries = Object.entries(headers);
        if (headerEntries.length > 0) {
            headerEntries.forEach(([key, value]) => {
                const listItem = document.createElement('li');
                listItem.innerHTML = `<strong>${this.escapeHtml(key)}:</strong> ${this.escapeHtml(value)}`;
                listItem.className = 'list-group-item';
                headersList.appendChild(listItem);
            });
        } else {
            const listItem = document.createElement('li');
            listItem.textContent = 'No headers available';
            listItem.className = 'list-group-item text-muted';
            headersList.appendChild(listItem);
        }

        // Body (truncated for display)
        const body = item.Body || '';
        const truncatedBody = body.length > 1000 ? body.substring(0, 1000) + '...' : body;
        document.getElementById('modalBody').textContent = truncatedBody || 'No body content';

        // Show the modal
        const modal = new bootstrap.Modal(document.getElementById('detailModal'));
        modal.show();
    }

    getStatusBadgeClass(statusCode) {
        if (!statusCode) return 'bg-secondary';
        if (statusCode >= 200 && statusCode < 300) return 'bg-success';
        if (statusCode >= 300 && statusCode < 400) return 'bg-info';
        if (statusCode >= 400 && statusCode < 500) return 'bg-warning';
        if (statusCode >= 500) return 'bg-danger';
        return 'bg-secondary';
    }

    formatBytes(bytes) {
        if (!bytes) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    escapeHtml(text) {
        if (!text) return '';
        const map = {
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            '"': '&quot;',
            "'": '&#039;'
        };
        return text.replace(/[&<>"']/g, function (m) { return map[m]; });
    }

    setupDataTable() {
        if (!$.fn.DataTable) {
            console.warn('DataTables not available');
            return;
        }

        try {
            // Prepare data for DataTables
            const tableData = this.probeResults.map(item => [
                item.InputURL || '',
                item.FinalURL || '',
                item.StatusCode || 0,
                item.Title || '',
                (item.Technologies || []).join(', '),
                item.WebServer || '',
                item.ContentType || '',
                item.ContentLength || 0,
                item.URLStatus || ''
            ]);

            this.table = $('#resultsTable').DataTable({
                data: tableData,
                columns: [
                    { title: "Input URL", className: "text-truncate", width: "20%" },
                    { title: "Final URL", className: "text-truncate", width: "20%" },
                    { title: "Status", width: "8%" },
                    { title: "Title", className: "text-truncate", width: "15%" },
                    { title: "Technologies", className: "text-truncate", width: "12%" },
                    { title: "Server", className: "text-truncate", width: "10%" },
                    { title: "Content Type", className: "text-truncate", width: "10%" },
                    { title: "Size", width: "8%" },
                    { title: "Diff Status", width: "8%" }
                ],
                pageLength: this.itemsPerPage,
                responsive: true,
                searching: true,
                ordering: true,
                info: true,
                lengthChange: true,
                lengthMenu: [[10, 25, 50, 100, -1], [10, 25, 50, 100, "All"]],
                language: {
                    search: "Search all columns:",
                    lengthMenu: "Show _MENU_ results per page",
                    info: "Showing _START_ to _END_ of _TOTAL_ results",
                    infoEmpty: "No results available",
                    infoFiltered: "(filtered from _MAX_ total results)",
                    paginate: {
                        first: "First",
                        last: "Last",
                        next: "Next",
                        previous: "Previous"
                    }
                },
                dom: '<"row"<"col-sm-12 col-md-6"l><"col-sm-12 col-md-6"f>>' +
                    '<"row"<"col-sm-12"tr>>' +
                    '<"row"<"col-sm-12 col-md-5"i><"col-sm-12 col-md-7"p>>',
                columnDefs: [
                    {
                        targets: [0, 1, 3, 4, 5, 6], // URL columns, title, tech, server, content type
                        render: function (data, type, row) {
                            if (type === 'display' && data && data.length > 50) {
                                return '<span title="' + data + '">' + data.substr(0, 47) + '...</span>';
                            }
                            return data;
                        }
                    },
                    {
                        targets: 2, // Status code column
                        render: function (data, type, row) {
                            if (type === 'display') {
                                const statusClass = data >= 200 && data < 300 ? 'bg-success' :
                                    data >= 300 && data < 400 ? 'bg-info' :
                                        data >= 400 && data < 500 ? 'bg-warning' :
                                            data >= 500 ? 'bg-danger' : 'bg-secondary';
                                return '<span class="badge ' + statusClass + '">' + (data || 'N/A') + '</span>';
                            }
                            return data;
                        }
                    },
                    {
                        targets: 7, // Content length column
                        render: function (data, type, row) {
                            if (type === 'display' && data) {
                                return this.formatBytes(data);
                            }
                            return data || 'N/A';
                        }.bind(this)
                    },
                    {
                        targets: 8, // Diff status column
                        render: function (data, type, row) {
                            if (type === 'display' && data) {
                                return '<span class="badge badge-info">' + data + '</span>';
                            }
                            return data || '';
                        }
                    }
                ],
                createdRow: (row, data, dataIndex) => {
                    $(row).css('cursor', 'pointer');
                    $(row).on('click', () => {
                        this.openDetailModal(this.probeResults[dataIndex]);
                    });
                }
            });

        } catch (error) {
            console.error('Error initializing DataTable:', error);
            this.setupManualPagination();
        }
    }

    setupManualPagination() {
        this.renderTable();
        this.setupPaginationControls();
        this.setupFilterControls();
    }

    updatePaginationControls() {
        const pagination = document.getElementById('paginationControls');
        if (!pagination) return;

        const startItem = ((this.currentPage - 1) * this.itemsPerPage) + 1;
        const endItem = Math.min(this.currentPage * this.itemsPerPage, this.totalItems);

        pagination.innerHTML = `
            <div class="d-flex justify-content-between align-items-center">
                <span class="text-muted">
                    Showing ${startItem}-${endItem} of ${this.totalItems} results
                </span>
                <div class="btn-group" role="group">
                    <button class="btn btn-outline-secondary" ${this.currentPage === 1 ? 'disabled' : ''} 
                            onclick="reportManager.goToPage(${this.currentPage - 1})">Previous</button>
                    <span class="btn btn-outline-secondary disabled">Page ${this.currentPage} of ${this.totalPages}</span>
                    <button class="btn btn-outline-secondary" ${this.currentPage === this.totalPages ? 'disabled' : ''} 
                            onclick="reportManager.goToPage(${this.currentPage + 1})">Next</button>
                </div>
            </div>
        `;
    }

    goToPage(page) {
        if (page >= 1 && page <= this.totalPages) {
            this.currentPage = page;
            this.renderTable();
        }
    }

    setupPaginationControls() {
        this.updatePaginationControls();
    }

    setupFilterControls() {
        // Global search
        const globalSearch = document.getElementById('globalSearch');
        if (globalSearch) {
            globalSearch.addEventListener('input', (e) => {
                this.globalFilter = e.target.value.toLowerCase();
                this.applyFilters();
            });
        }

        // Status filter
        const statusFilter = document.getElementById('statusFilter');
        if (statusFilter) {
            statusFilter.addEventListener('change', (e) => {
                this.statusFilter = e.target.value;
                this.applyFilters();
            });
        }

        // Technology filter
        const techFilter = document.getElementById('techFilter');
        if (techFilter) {
            techFilter.addEventListener('input', (e) => {
                this.techFilter = e.target.value.toLowerCase();
                this.applyFilters();
            });
        }

        // Hostname filter
        const hostnameFilter = document.getElementById('hostnameFilter');
        if (hostnameFilter) {
            hostnameFilter.addEventListener('change', (e) => {
                this.hostnameFilter = e.target.value;
                this.applyFilters();
            });
        }

        // Root target filter
        const rootTargetFilter = document.getElementById('rootTargetFilter');
        if (rootTargetFilter) {
            rootTargetFilter.addEventListener('change', (e) => {
                this.rootTargetFilter = e.target.value;
                this.applyFilters();
            });
        }
    }

    applyFilters() {
        // Implementation for manual filtering when DataTables is not available
        console.log('Applying manual filters...');
    }
}

// Initialize the report manager when the DOM is loaded
document.addEventListener('DOMContentLoaded', function () {
    // ProbeResultsData should be injected by the Go template
    const reportManager = new ReportManager(window.ProbeResultsData || []);
    window.reportManager = reportManager; // Make it globally accessible for inline onclick handlers
});