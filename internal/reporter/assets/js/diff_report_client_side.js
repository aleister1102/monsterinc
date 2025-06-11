// Diff Report Client-Side JavaScript

class DiffRenderer {
    constructor() {
        this.currentPage = 1;
        this.itemsPerPage = 10;
        this.totalItems = window.diffData.length;
        this.totalPages = Math.ceil(this.totalItems / this.itemsPerPage);
        this.init();
    }

    init() {
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
    }

    bindEvents() {
        // Pagination controls
        document.getElementById('itemsPerPage').addEventListener('change', (e) => {
            this.itemsPerPage = parseInt(e.target.value);
            this.totalPages = Math.ceil(this.totalItems / this.itemsPerPage);
            this.currentPage = 1;
            this.render();
        });

        // Expand/Collapse all
        document.getElementById('expandAll').addEventListener('click', () => {
            document.querySelectorAll('.diff-content').forEach(content => {
                content.classList.add('show');
            });
            document.querySelectorAll('.collapse-icon').forEach(icon => {
                icon.classList.add('rotated');
            });
        });

        document.getElementById('collapseAll').addEventListener('click', () => {
            document.querySelectorAll('.diff-content').forEach(content => {
                content.classList.remove('show');
            });
            document.querySelectorAll('.collapse-icon').forEach(icon => {
                icon.classList.remove('rotated');
            });
        });
    }

    render() {
        this.renderDiffs();
        this.renderPagination();
    }

    renderDiffs() {
        const container = document.getElementById('diff-container');
        const startIndex = (this.currentPage - 1) * this.itemsPerPage;
        const endIndex = Math.min(startIndex + this.itemsPerPage, this.totalItems);
        const pageData = window.diffData.slice(startIndex, endIndex);

        if (pageData.length === 0) {
            container.innerHTML = '<div class="no-diff-notice">No diff results to display.</div>';
            return;
        }

        container.innerHTML = pageData.map((diff, index) => this.createDiffHTML(diff, startIndex + index)).join('');
        
        // Bind click events for expand/collapse
        container.querySelectorAll('.diff-header').forEach((header, index) => {
            header.addEventListener('click', () => {
                const content = header.nextElementSibling;
                const icon = header.querySelector('.collapse-icon');
                
                content.classList.toggle('show');
                icon.classList.toggle('rotated');
            });
        });

        // Bind copy URL events
        container.querySelectorAll('.copy-url-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                const url = btn.dataset.url;
                navigator.clipboard.writeText(url).then(() => {
                    const originalHTML = btn.innerHTML;
                    btn.innerHTML = '<i class="fas fa-check"></i>';
                    setTimeout(() => {
                        btn.innerHTML = originalHTML;
                    }, 2000);
                }).catch(err => {
                    console.error('Failed to copy URL:', err);
                });
            });
        });
    }

    createDiffHTML(diff, index) {
        const timestamp = new Date(diff.timestamp).toLocaleString();
        const diffHTML = this.generateDiffHTML(diff.diffs);

        return `
            <div class="diff-item" data-index="${index}">
                <div class="diff-header">
                    <div>
                        <div class="diff-url">${this.escapeHtml(diff.url)}</div>
                        <div class="diff-meta">
                            ${timestamp} | ${diff.content_type || 'unknown'} | 
                            Changes: ${diff.summary || 'No summary available'}
                            ${diff.extracted_paths && diff.extracted_paths.length > 0 ? 
                                `<span class="ms-2 badge rounded-pill bg-info"><i class="fas fa-sitemap me-1"></i> ${diff.extracted_paths.length} ${diff.extracted_paths.length === 1 ? 'path' : 'paths'}</span>` : 
                                ''
                            }
                        </div>
                    </div>
                    <div class="diff-controls">
                        <button class="btn btn-sm btn-outline-light copy-url-btn" data-url="${this.escapeHtml(diff.url)}" title="Copy URL">
                            <i class="fas fa-copy"></i>
                        </button>
                        <i class="fas fa-chevron-down collapse-icon"></i>
                    </div>
                </div>
                <div class="diff-content">
                    ${diff.error_message ? 
                        `<div class="error-message">${this.escapeHtml(diff.error_message)}</div>` : 
                        (diffHTML || '<div class="no-diff-notice">No diff content available</div>')
                    }
                    ${this.createExtractedPathsHTML(diff.extracted_paths)}
                </div>
            </div>
        `;
    }

    generateDiffHTML(diffs) {
        if (!diffs || diffs.length === 0) return '';
        
        let htmlBuilder = '';
        
        // Check if this is a large single diff (like minified JS)
        const totalLength = diffs.reduce((sum, d) => sum + (d.text ? d.text.length : 0), 0);
        const isLargeContent = totalLength > 10000 && diffs.length <= 3;
        
        for (let i = 0; i < diffs.length; i++) {
            const d = diffs[i];
            if (!d.text) continue;
            
            // Escape HTML characters to prevent XSS
            let escapedText = this.escapeHtml(d.text);
            
            // For large content, add line breaks for better readability
            if (isLargeContent && escapedText.length > 120) {
                escapedText = this.formatLargeContent(escapedText);
            }
            
            switch (d.operation) {
                case 1: // DiffInsert
                    if (isLargeContent) {
                        htmlBuilder += `<div class="diff-line diff-line-insert"><span class="diff-add" style="background:#e6ffe6; text-decoration: none; display: block; padding: 2px 4px; margin: 1px 0;">${escapedText}</span></div>`;
                    } else {
                        htmlBuilder += `<span class="diff-add" style="background:#e6ffe6; text-decoration: none;">${escapedText}</span>`;
                    }
                    break;
                case -1: // DiffDelete
                    if (isLargeContent) {
                        htmlBuilder += `<div class="diff-line diff-line-delete"><span class="diff-remove" style="background:#f8d7da; text-decoration: none; display: block; padding: 2px 4px; margin: 1px 0;">${escapedText}</span></div>`;
                    } else {
                        htmlBuilder += `<span class="diff-remove" style="background:#f8d7da; text-decoration: none;">${escapedText}</span>`;
                    }
                    break;
                case 0: // DiffEqual
                    // For large equal content, truncate in middle to show context
                    if (isLargeContent && escapedText.length > 200) {
                        const start = escapedText.substring(0, 100);
                        const end = escapedText.substring(escapedText.length - 100);
                        const truncated = `${start}<span style="color: #666; font-style: italic;">... [${escapedText.length - 200} characters truncated] ...</span>${end}`;
                        htmlBuilder += truncated;
                    } else {
                        htmlBuilder += escapedText;
                    }
                    break;
            }
            
            // Add spacing between diffs for readability
            if (isLargeContent && i < diffs.length - 1) {
                htmlBuilder += '\n';
            }
        }
        
        // Wrap in a container for better formatting
        return `<div class="diff-content-wrapper">${htmlBuilder}</div>`;
    }
    
    formatLargeContent(content) {
        // Don't format if content already has line breaks
        if (content.includes('\n')) {
            return content;
        }
        
        // For very long single lines (like minified JS), add breaks at logical points
        let result = '';
        const chunkSize = 120;
        
        for (let i = 0; i < content.length; i += chunkSize) {
            const end = Math.min(i + chunkSize, content.length);
            const chunk = content.substring(i, end);
            result += chunk;
            
            // Add line break if not at the end
            if (end < content.length) {
                result += '\n';
            }
        }
        
        return result;
    }

    renderPagination() {
        const topPagination = document.getElementById('pagination-top');
        const bottomPagination = document.getElementById('pagination-bottom');
        
        if (this.totalPages <= 1) {
            topPagination.style.display = 'none';
            bottomPagination.style.display = 'none';
            return;
        }

        const paginationHTML = this.createPaginationHTML();
        topPagination.innerHTML = paginationHTML;
        bottomPagination.innerHTML = paginationHTML;
        topPagination.style.display = 'flex';
        bottomPagination.style.display = 'flex';

        // Bind pagination events
        [topPagination, bottomPagination].forEach(container => {
            container.querySelectorAll('.page-link').forEach(link => {
                link.addEventListener('click', (e) => {
                    e.preventDefault();
                    const page = parseInt(link.dataset.page);
                    if (page && page !== this.currentPage) {
                        this.currentPage = page;
                        this.render();
                        this.scrollToTop();
                    }
                });
            });
        });
    }

    createPaginationHTML() {
        let html = '<nav><ul class="pagination">';
        
        // Previous button
        html += `<li class="page-item ${this.currentPage === 1 ? 'disabled' : ''}">
            <a class="page-link" href="#" data-page="${this.currentPage - 1}">&laquo;</a>
        </li>`;

        // Page numbers
        const startPage = Math.max(1, this.currentPage - 2);
        const endPage = Math.min(this.totalPages, this.currentPage + 2);

        if (startPage > 1) {
            html += `<li class="page-item"><a class="page-link" href="#" data-page="1">1</a></li>`;
            if (startPage > 2) {
                html += '<li class="page-item disabled"><span class="page-link">...</span></li>';
            }
        }

        for (let i = startPage; i <= endPage; i++) {
            html += `<li class="page-item ${i === this.currentPage ? 'active' : ''}">
                <a class="page-link" href="#" data-page="${i}">${i}</a>
            </li>`;
        }

        if (endPage < this.totalPages) {
            if (endPage < this.totalPages - 1) {
                html += '<li class="page-item disabled"><span class="page-link">...</span></li>';
            }
            html += `<li class="page-item"><a class="page-link" href="#" data-page="${this.totalPages}">${this.totalPages}</a></li>`;
        }

        // Next button
        html += `<li class="page-item ${this.currentPage === this.totalPages ? 'disabled' : ''}">
            <a class="page-link" href="#" data-page="${this.currentPage + 1}">&raquo;</a>
        </li>`;

        html += '</ul></nav>';
        return html;
    }

    setupScrollToTop() {
        const toTopBtn = document.getElementById('toTopBtn');
        
        window.addEventListener('scroll', () => {
            if (document.body.scrollTop > 200 || document.documentElement.scrollTop > 200) {
                toTopBtn.style.display = 'block';
            } else {
                toTopBtn.style.display = 'none';
            }
        });

        toTopBtn.addEventListener('click', () => {
            this.scrollToTop();
        });
    }

    scrollToTop() {
        window.scrollTo({
            top: 0,
            behavior: 'smooth'
        });
    }

    createExtractedPathsHTML(extractedPaths) {
        if (!extractedPaths || extractedPaths.length === 0) {
            return '';
        }

        let html = `
            <div class="extracted-paths mt-3">
                <h6><i class="fas fa-sitemap me-2"></i><strong>Extracted Paths (${extractedPaths.length})</strong></h6>
                <div class="extracted-paths-list">`;

        extractedPaths.forEach(path => {
            html += `
                <div class="extracted-path-item mb-2 p-2 border rounded">
                    <div class="path-type">
                        <span class="badge bg-secondary">${this.escapeHtml(path.type || 'unknown')}</span>
                    </div>
                    <div class="path-raw mt-1">
                        <code class="bg-light px-2 py-1 rounded d-block">${this.escapeHtml(path.extracted_raw_path || '')}</code>
                    </div>
                    <div class="path-absolute mt-1">
                        <a href="${this.escapeHtml(path.extracted_absolute_url || '')}" target="_blank" class="text-break">
                            ${this.escapeHtml(path.extracted_absolute_url || '')}
                        </a>
                    </div>
                </div>`;
        });

        html += `
                </div>
            </div>`;

        return html;
    }

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    new DiffRenderer();
});