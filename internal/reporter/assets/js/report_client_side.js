class ReportRenderer {
    constructor() {
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
        this.init();
    }

    init() {
        this.hideLoading();
        this.showControls();
        this.bindEvents();
        this.render();
        this.setupScrollToTop();
    }

    // Hàm rút gọn URL hiển thị phần đầu và phần cuối
    truncateURL(url, maxLength = 50) {
        if (!url || url.length <= maxLength) {
            return url || '';
        }
        
        const prefixLength = Math.floor(maxLength * 0.4); // 40% cho phần đầu
        const suffixLength = Math.floor(maxLength * 0.4); // 40% cho phần cuối
        
        const prefix = url.substring(0, prefixLength);
        const suffix = url.substring(url.length - suffixLength);
        
        return `${prefix}...${suffix}`;
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
        // Search input with debounce
        let searchTimeout;
        document.getElementById('globalSearchInput').addEventListener('input', (e) => {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => {
                this.filters.global = e.target.value.toLowerCase();
                this.applyFilters();
            }, 300);
        });

        // Filter dropdowns
        document.getElementById('rootURLFilter').addEventListener('change', (e) => {
            this.filters.hostname = e.target.value;
            this.applyFilters();
        });

        document.getElementById('statusCodeFilter').addEventListener('change', (e) => {
            this.filters.statusCode = e.target.value;
            this.applyFilters();
        });

        document.getElementById('contentTypeFilter').addEventListener('change', (e) => {
            this.filters.contentType = e.target.value;
            this.applyFilters();
        });

        document.getElementById('technologyFilter').addEventListener('change', (e) => {
            this.filters.technology = e.target.value;
            this.applyFilters();
        });

        document.getElementById('diffStatusFilter').addEventListener('change', (e) => {
            this.filters.diffStatus = e.target.value;
            this.applyFilters();
        });

        // Items per page - Top
        document.getElementById('itemsPerPageSelect').addEventListener('change', (e) => {
            this.itemsPerPage = parseInt(e.target.value);
            this.totalPages = Math.ceil(this.totalItems / this.itemsPerPage);
            this.currentPage = 1;
            // Sync with bottom dropdown
            document.getElementById('itemsPerPageSelectBottom').value = e.target.value;
            this.render();
        });

        // Items per page - Bottom
        document.getElementById('itemsPerPageSelectBottom').addEventListener('change', (e) => {
            this.itemsPerPage = parseInt(e.target.value);
            this.totalPages = Math.ceil(this.totalItems / this.itemsPerPage);
            this.currentPage = 1;
            // Sync with top dropdown
            document.getElementById('itemsPerPageSelect').value = e.target.value;
            this.render();
        });

        // Table sorting
        document.querySelectorAll('.sortable').forEach(th => {
            th.addEventListener('click', () => {
                const column = th.dataset.colName;
                this.sortData(column);
            });
        });
    }

    applyFilters() {
        this.filteredData = this.data.filter(item => {
            // Global search
            if (this.filters.global) {
                const searchText = this.filters.global;
                const searchableText = [
                    item.InputURL,
                    item.FinalURL,
                    item.Title,
                    item.ContentType,
                    item.WebServer,
                    item.URLStatus,
                    ...(item.Technologies || []).map(tech => 
                        typeof tech === 'object' ? tech.Name : tech
                    )
                ].join(' ').toLowerCase();
                
                if (!searchableText.includes(searchText)) {
                    return false;
                }
            }

            // Hostname filter
            if (this.filters.hostname) {
                const hostname = this.extractHostname(item.InputURL);
                if (hostname !== this.filters.hostname) {
                    return false;
                }
            }

            // Status code filter
            if (this.filters.statusCode) {
                if (item.StatusCode.toString() !== this.filters.statusCode) {
                    return false;
                }
            }

            // Content type filter
            if (this.filters.contentType) {
                if (!item.ContentType || !item.ContentType.includes(this.filters.contentType)) {
                    return false;
                }
            }

            // Technology filter
            if (this.filters.technology) {
                const technologies = item.Technologies || [];
                const hasTech = technologies.some(tech => {
                    const techName = typeof tech === 'object' ? tech.Name : tech;
                    return techName && techName.toLowerCase().includes(this.filters.technology.toLowerCase());
                });
                if (!hasTech) {
                    return false;
                }
            }

            // Diff status filter
            if (this.filters.diffStatus) {
                const status = item.URLStatus || item.diff_status || '';
                if (status !== this.filters.diffStatus) {
                    return false;
                }
            }

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
            let aVal = a[column];
            let bVal = b[column];

            // Handle special cases
            if (column === 'hostname') {
                aVal = this.extractHostname(a.InputURL);
                bVal = this.extractHostname(b.InputURL);
            }

            // Convert to string for comparison
            aVal = aVal ? aVal.toString().toLowerCase() : '';
            bVal = bVal ? bVal.toString().toLowerCase() : '';

            if (this.currentSort.direction === 'asc') {
                return aVal.localeCompare(bVal);
            } else {
                return bVal.localeCompare(aVal);
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
        this.populateFilterOptions();
    }

    renderTable() {
        const tbody = document.querySelector('#resultsTable tbody');
        if (!tbody) return;

        if (this.filteredData.length === 0) {
            tbody.innerHTML = '<tr><td colspan="7" class="text-center text-muted py-4">No results found</td></tr>';
            document.getElementById('paginationContainer').style.display = 'none';
            return;
        }

        const startIndex = (this.currentPage - 1) * this.itemsPerPage;
        const endIndex = Math.min(startIndex + this.itemsPerPage, this.filteredData.length);
        const pageData = this.filteredData.slice(startIndex, endIndex);

        document.getElementById('paginationContainer').style.display = 'block';

        tbody.innerHTML = pageData.map((item, index) => this.createTableRow(item, startIndex + index)).join('');
        
        // Bind view details events
        tbody.querySelectorAll('.view-details-btn').forEach((btn, index) => {
            btn.addEventListener('click', () => {
                this.showDetails(pageData[index]);
            });
        });
    }

    createTableRow(item, index) {
        const statusClass = `status-${item.StatusCode}`;
        const diffStatusClass = `diff-status-${(item.URLStatus || item.diff_status || '').toLowerCase()}`;
        
        // Hợp nhất URL: ưu tiên FinalURL, nếu không có thì dùng InputURL
        const displayURL = item.FinalURL || item.InputURL;
        const truncatedURL = this.truncateURL(displayURL, 60);
        
        return `
            <tr data-index="${index}">
                <td>
                    <div class="text-truncate" style="max-width: 300px;" title="${this.escapeHtml(displayURL || '')}">
                        ${displayURL ? `<a href="${this.escapeHtml(displayURL)}" target="_blank" class="text-decoration-none">${this.escapeHtml(truncatedURL)}</a>` : '-'}
                    </div>
                </td>
                <td class="text-center">
                    <span class="${diffStatusClass}">${this.escapeHtml(item.URLStatus || item.diff_status || '')}</span>
                </td>
                <td>
                    <span class="${statusClass}">${item.StatusCode || '-'}</span>
                </td>
                <td class="hide-on-mobile">
                    <div class="text-truncate" style="max-width: 150px;" title="${this.escapeHtml(item.Title || '')}">
                        ${this.escapeHtml(item.Title || '-')}
                    </div>
                </td>
                <td class="hide-on-mobile">
                    <div class="text-truncate" style="max-width: 180px;" title="${this.escapeHtml(item.ContentType || '')}">
                        ${this.escapeHtml(item.ContentType || '-')}
                    </div>
                </td>
                <td class="hide-on-mobile">
                    ${this.renderTechnologies(item.Technologies)}
                </td>
                <td>
                    <button class="btn btn-outline-primary btn-sm view-details-btn" type="button">
                        <i class="fas fa-eye"></i> View
                    </button>
                </td>
            </tr>
        `;
    }

    renderTechnologies(technologies) {
        if (!technologies || !Array.isArray(technologies) || technologies.length === 0) {
            return '<span class="text-muted">-</span>';
        }

        return technologies.map(tech => {
            const techName = typeof tech === 'object' ? tech.Name : tech;
            return `<span class="tech-tag me-1 mb-1">${this.escapeHtml(techName)}</span>`;
        }).join('');
    }

    showDetails(item) {
        const modal = new bootstrap.Modal(document.getElementById('detailsModal'));
        
        if (item) {
            // Helper function to safely set element content
            const safeSetContent = (id, value) => {
                const element = document.getElementById(id);
                if (element) {
                    element.textContent = value || 'N/A';
                }
            };
            
            // URL Information - Try multiple field name formats
            safeSetContent('details-input-url', item.input_url || item.InputURL || item.url);
            safeSetContent('details-final-url', item.final_url || item.FinalURL);
            safeSetContent('details-root-target', item.root_target || item.RootTargetURL);
            safeSetContent('details-diff-status', item.diff_status || item.DiffStatus || item.URLStatus || item.url_status || 'unknown');
            safeSetContent('details-timestamp', item.timestamp || item.Timestamp);
            
            // Response Information
            safeSetContent('details-method', item.method || item.Method);
            safeSetContent('details-status-code', item.status_code || item.StatusCode);
            safeSetContent('details-content-type', item.content_type || item.ContentType);
            safeSetContent('details-content-length', item.content_length || item.ContentLength);
            safeSetContent('details-web-server', item.web_server || item.WebServer);
            
            // Network Information with multiple formats
            safeSetContent('details-ips', 
                Array.isArray(item.ips) ? item.ips.join(', ') : 
                Array.isArray(item.IPs) ? item.IPs.join(', ') : 
                item.ips || item.IPs || 'N/A'
            );
            safeSetContent('details-cnames', 
                Array.isArray(item.cnames) ? item.cnames.join(', ') : 
                Array.isArray(item.CNAMEs) ? item.CNAMEs.join(', ') : 
                item.cnames || item.CNAMEs || 'N/A'
            );
            safeSetContent('details-asn', item.asn || item.ASN);
            safeSetContent('details-asn-org', item.asn_org || item.ASNOrg);
            
            // Technologies
            const techElement = document.getElementById('details-technologies');
            if (techElement) {
                const technologies = item.technologies || item.Technologies || [];
                if (Array.isArray(technologies) && technologies.length > 0) {
                    techElement.innerHTML = technologies.map(tech => {
                        // Handle both string and object formats
                        const techName = typeof tech === 'object' ? (tech.Name || tech.name) : tech;
                        const techVersion = typeof tech === 'object' ? (tech.Version || tech.version) : '';
                        const displayText = techVersion ? `${techName} ${techVersion}` : techName;
                        return `<span class="badge bg-secondary me-1 mb-1">${this.escapeHtml(displayText)}</span>`;
                    }).join('');
                } else {
                    techElement.textContent = 'No technologies detected';
                }
            }
            
            // Headers
            const headersElement = document.getElementById('details-headers');
            if (headersElement) {
                const headers = item.headers || item.Headers;
                if (headers && typeof headers === 'object') {
                    headersElement.innerHTML = Object.entries(headers).map(([key, value]) => 
                        `<tr><td><strong>${this.escapeHtml(key)}</strong></td><td>${this.escapeHtml(value)}</td></tr>`
                    ).join('');
                } else {
                    headersElement.innerHTML = '<tr><td colspan="2" class="text-muted">No headers available</td></tr>';
                }
            }
            
            // Response Body
            const bodyElement = document.getElementById('details-body');
            if (bodyElement) {
                bodyElement.textContent = item.Body || item.body || 'No response body available';
            }
        }
        
        modal.show();
    }

    extractHostname(url) {
        if (!url) return '';
        try {
            return new URL(url).hostname;
        } catch {
            return '';
        }
    }

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    renderPagination() {
        this.renderPaginationForElement('paginationList');
        this.renderPaginationForElement('paginationListBottom');
        this.updatePaginationInfo();
    }

    renderPaginationForElement(elementId) {
        const paginationElement = document.getElementById(elementId);
        if (!paginationElement) return;

        if (this.totalPages <= 1) {
            paginationElement.innerHTML = '';
            return;
        }

        let paginationHTML = '';
        
        // Previous button
        paginationHTML += `
            <li class="page-item ${this.currentPage === 1 ? 'disabled' : ''}">
                <a class="page-link" href="#" data-page="${this.currentPage - 1}">Previous</a>
            </li>
        `;

        // Page numbers
        const startPage = Math.max(1, this.currentPage - 2);
        const endPage = Math.min(this.totalPages, this.currentPage + 2);

        if (startPage > 1) {
            paginationHTML += '<li class="page-item"><a class="page-link" href="#" data-page="1">1</a></li>';
            if (startPage > 2) {
                paginationHTML += '<li class="page-item disabled"><span class="page-link">...</span></li>';
            }
        }

        for (let i = startPage; i <= endPage; i++) {
            paginationHTML += `
                <li class="page-item ${i === this.currentPage ? 'active' : ''}">
                    <a class="page-link" href="#" data-page="${i}">${i}</a>
                </li>
            `;
        }

        if (endPage < this.totalPages) {
            if (endPage < this.totalPages - 1) {
                paginationHTML += '<li class="page-item disabled"><span class="page-link">...</span></li>';
            }
            paginationHTML += `<li class="page-item"><a class="page-link" href="#" data-page="${this.totalPages}">${this.totalPages}</a></li>`;
        }

        // Next button
        paginationHTML += `
            <li class="page-item ${this.currentPage === this.totalPages ? 'disabled' : ''}">
                <a class="page-link" href="#" data-page="${this.currentPage + 1}">Next</a>
            </li>
        `;

        paginationElement.innerHTML = paginationHTML;

        // Bind pagination events
        paginationElement.querySelectorAll('a.page-link').forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const page = parseInt(e.target.dataset.page);
                if (page && page !== this.currentPage && page >= 1 && page <= this.totalPages) {
                    this.currentPage = page;
                    this.render();
                }
            });
        });
    }

    updatePaginationInfo() {
        const startIndex = (this.currentPage - 1) * this.itemsPerPage + 1;
        const endIndex = Math.min(this.currentPage * this.itemsPerPage, this.totalItems);
        
        // Update top pagination info
        const showingStart = document.getElementById('showingStart');
        const showingEnd = document.getElementById('showingEnd');
        const totalItems = document.getElementById('totalItems');
        
        if (showingStart) showingStart.textContent = startIndex;
        if (showingEnd) showingEnd.textContent = endIndex;
        if (totalItems) totalItems.textContent = this.totalItems;
        
        // Update bottom pagination info
        const showingStartBottom = document.getElementById('showingStartBottom');
        const showingEndBottom = document.getElementById('showingEndBottom');
        const totalItemsBottom = document.getElementById('totalItemsBottom');
        
        if (showingStartBottom) showingStartBottom.textContent = startIndex;
        if (showingEndBottom) showingEndBottom.textContent = endIndex;
        if (totalItemsBottom) totalItemsBottom.textContent = this.totalItems;
    }

    updateResultsInfo() {
        const infoElement = document.getElementById('resultsInfo');
        if (!infoElement) return;

        const startIndex = (this.currentPage - 1) * this.itemsPerPage + 1;
        const endIndex = Math.min(this.currentPage * this.itemsPerPage, this.totalItems);
        
        infoElement.textContent = `Showing ${startIndex}-${endIndex} of ${this.totalItems} results`;
    }

    populateFilterOptions() {
        // Populate hostname filter
        const hostnameFilter = document.getElementById('rootURLFilter');
        if (hostnameFilter) {
            const hostnames = [...new Set(this.data.map(item => this.extractHostname(item.InputURL)).filter(h => h))];
            const currentValue = hostnameFilter.value;
            
            hostnameFilter.innerHTML = '<option value="">All Hostnames</option>' +
                hostnames.map(hostname => `<option value="${hostname}">${hostname}</option>`).join('');
            
            hostnameFilter.value = currentValue;
        }

        // Populate status code filter
        const statusFilter = document.getElementById('statusCodeFilter');
        if (statusFilter) {
            const statusCodes = [...new Set(this.data.map(item => item.StatusCode).filter(s => s))];
            const currentValue = statusFilter.value;
            
            statusFilter.innerHTML = '<option value="">All Status Codes</option>' +
                statusCodes.sort((a, b) => a - b).map(code => `<option value="${code}">${code}</option>`).join('');
            
            statusFilter.value = currentValue;
        }

        // Populate content type filter
        const contentTypeFilter = document.getElementById('contentTypeFilter');
        if (contentTypeFilter) {
            const contentTypes = [...new Set(this.data.map(item => {
                if (!item.ContentType) return null;
                return item.ContentType.split(';')[0].trim();
            }).filter(ct => ct))];
            const currentValue = contentTypeFilter.value;
            
            contentTypeFilter.innerHTML = '<option value="">All Content Types</option>' +
                contentTypes.sort().map(type => `<option value="${type}">${type}</option>`).join('');
            
            contentTypeFilter.value = currentValue;
        }

        // Populate diff status filter
        const diffStatusFilter = document.getElementById('diffStatusFilter');
        if (diffStatusFilter) {
            const statuses = [...new Set(this.data.map(item => item.URLStatus || item.diff_status).filter(s => s))];
            const currentValue = diffStatusFilter.value;
            
            diffStatusFilter.innerHTML = '<option value="">All Statuses</option>' +
                statuses.sort().map(status => `<option value="${status}">${status}</option>`).join('');
            
            diffStatusFilter.value = currentValue;
        }
    }

    setupScrollToTop() {
        const scrollBtn = document.getElementById('toTopBtn');
        if (!scrollBtn) return;

        window.addEventListener('scroll', () => {
            if (window.pageYOffset > 300) {
                scrollBtn.style.display = 'block';
            } else {
                scrollBtn.style.display = 'none';
            }
        });

        scrollBtn.addEventListener('click', () => {
            window.scrollTo({ top: 0, behavior: 'smooth' });
        });
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    // Initialize the report renderer
    window.reportRenderer = new ReportRenderer();
});