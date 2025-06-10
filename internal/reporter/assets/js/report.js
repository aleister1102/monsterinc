// MonsterInc Report interactivity script using jQuery

$(document).ready(function () {
    const $resultsTable = $('#resultsTable');
    const $tableBody = $resultsTable.find('tbody');
    let allRowsData = [];

    // Load data from Go template injection
    if (typeof window.reportSettings !== 'undefined' && window.reportSettings.initialProbeResults) {
        allRowsData = window.reportSettings.initialProbeResults;
        console.log("Loaded data from Go template:", allRowsData.length, "items");
        console.log("First item:", allRowsData[0]);
    } else {
        console.warn("No data found in window.reportSettings.initialProbeResults");
        console.log("window.reportSettings:", window.reportSettings);
    }

    // const $globalSearchInput = $('#globalSearchInput'); // Global search disabled
    const $globalSearchInput = $('#globalSearchInput'); // Global search enabled
    const $rootURLFilter = $('#rootURLFilter'); // New filter input for root URL
    const $statusCodeFilter = $('#statusCodeFilter');
    const $contentTypeFilter = $('#contentTypeFilter');
    const $techFilterInput = $('#techFilterInput');
    // const $targetFilterInput = $('#targetFilterInput'); // This was likely a typo or old, RootURLFilter is used now
    const $urlStatusFilter = $('#urlStatusFilter');
    const $paginationControls = $('#paginationControls');
    const $itemsPerPageSelect = $('#itemsPerPageSelect');
    const $resultsCountInfo = $('#resultsCountInfo');
    const $clearAllFiltersBtn = $('#clearAllFiltersBtn'); // Added Clear All Filters button

    let itemsPerPage = parseInt($itemsPerPageSelect.val()) || 10;
    let currentPage = 1;
    let currentSortColumn = null;
    let currentSortDirection = 'asc';
    let currentFilters = {
        // globalSearch: '', // Global search disabled
        globalSearch: '', // Global search enabled
        rootURL: '', // Added for root URL filtering
        statusCode: '',
        contentType: '',
        tech: '',
        urlStatus: ''
    };
    let filteredAndSortedData = [...allRowsData];

    // --- Helper: Truncate text ---
    function truncateText(text, maxLength) {
        if (text && text.length > maxLength) {
            return text.substring(0, maxLength - 3) + "...";
        }
        return text || '';
    }

    // --- Render Table Rows from Data ---
    function renderTableRows(dataToRender) {
        $tableBody.empty();
        if (!dataToRender || dataToRender.length === 0) {
            const colCount = $resultsTable.find('thead th').length || 9; // Updated to match actual template columns
            $tableBody.append(`<tr><td colspan="${colCount}" class="text-center">No results match your filters.</td></tr>`);
            return;
        }

        $.each(dataToRender, function (index, pr) {
            const originalIndex = allRowsData.indexOf(pr);
            const $row = $('<tr></tr>')
                .addClass(pr.IsSuccess ? (pr.StatusCode ? `status-${pr.StatusCode}` : '') : 'table-danger')
                .attr('data-result-index', originalIndex);

            // IMPORTANT: Keep this order exactly matching the <thead> in report.html.tmpl
            // 1. Input URL
            $row.append($('<td></td>').addClass('truncate-url').attr('title', pr.InputURL).html(pr.InputURL ? `<a href="${pr.InputURL}" target="_blank">${truncateText(pr.InputURL, 50)}</a>` : '-'));
            // 2. Final URL  
            $row.append($('<td></td>').addClass('truncate-url').attr('title', pr.FinalURL).html(pr.FinalURL ? `<a href="${pr.FinalURL}" target="_blank">${truncateText(pr.FinalURL, 50)}</a>` : '-'));
            // 3. Diff Status
            $row.append($('<td></td>').addClass('hide-on-mobile').html(pr.diff_status ? `<span class="diff-status-${pr.diff_status.toLowerCase()}">${pr.diff_status}</span>` : '-'));
            // 4. Status Code
            $row.append($('<td></td>').html(pr.StatusCode ? `<span class="${pr.StatusCode ? `status-${pr.StatusCode}` : ''}">${pr.StatusCode}</span>` : (pr.Error ? 'ERR' : '-')));
            // 5. Title
            $row.append($('<td></td>').addClass('truncate-title hide-on-small').attr('title', pr.Title).text(truncateText(pr.Title, 70) || '-'));
            // 6. Technologies
            const techString = Array.isArray(pr.Technologies) ? pr.Technologies.join(', ') : '';
            const techHtml = Array.isArray(pr.Technologies) && pr.Technologies.length > 0 ?
                pr.Technologies.map(tech => `<span class="tech-tag">${tech}</span>`).join('') : '-';
            $row.append($('<td></td>').addClass('truncate-techs hide-on-medium').attr('title', techString).html(techHtml));
            // 7. Content Type
            $row.append($('<td></td>').addClass('hide-on-mobile').text(pr.ContentType || '-'));
            // 9. Details button
            $row.append($('<td><button class="btn btn-sm btn-outline-primary view-details-btn" data-bs-toggle="modal" data-bs-target="#detailsModal"><i class="fas fa-eye me-1"></i>View</button></td>'));

            $tableBody.append($row);
        });
    }

    // --- Helper: Extract hostname from URL ---
    function extractHostname(url) {
        try {
            const urlObj = new URL(url);
            return urlObj.hostname;
        } catch (e) {
            return '';
        }
    }

    // --- Filtering Logic ---
    function filterData(data) {
        const globalSearchTerm = currentFilters.globalSearch.toLowerCase(); // Global search enabled
        const rootURL = currentFilters.rootURL;
        const statusCode = currentFilters.statusCode;
        const contentType = currentFilters.contentType.toLowerCase();
        const techTerm = currentFilters.tech.toLowerCase();
        const urlStatusTerm = currentFilters.urlStatus.toLowerCase();

        return data.filter(pr => {
            if (rootURL) {
                const inputHostname = extractHostname(pr.InputURL);
                if (!inputHostname || inputHostname !== rootURL) {
                    return false;
                }
            }

            // Global search enabled
            let matchesGlobal = true;
            if (globalSearchTerm) {
                matchesGlobal = (
                    (pr.InputURL && pr.InputURL.toLowerCase().includes(globalSearchTerm)) ||
                    (pr.FinalURL && pr.FinalURL.toLowerCase().includes(globalSearchTerm)) ||
                    (pr.Title && pr.Title.toLowerCase().includes(globalSearchTerm)) ||

                    (pr.ContentType && pr.ContentType.toLowerCase().includes(globalSearchTerm)) ||
                    (Array.isArray(pr.Technologies) && pr.Technologies.join(', ').toLowerCase().includes(globalSearchTerm)) ||
                    (Array.isArray(pr.IPs) && pr.IPs.join(', ').toLowerCase().includes(globalSearchTerm)) ||
                    (Array.isArray(pr.CNAMEs) && pr.CNAMEs.join(', ').toLowerCase().includes(globalSearchTerm)) ||
                    (pr.ASNOrg && pr.ASNOrg.toLowerCase().includes(globalSearchTerm)) ||
                    (pr.Error && pr.Error.toLowerCase().includes(globalSearchTerm))
                );
            }
            if (!matchesGlobal) return false;

            if (statusCode && (!pr.StatusCode || pr.StatusCode.toString() !== statusCode)) return false;
            if (contentType && (!pr.ContentType || !pr.ContentType.toLowerCase().startsWith(contentType))) return false;
            if (urlStatusTerm && (!pr.diff_status || pr.diff_status.toLowerCase() !== urlStatusTerm)) return false;

            if (techTerm) {
                const techString = Array.isArray(pr.Technologies) ? pr.Technologies.join(', ').toLowerCase() : "";
                const searchTechs = techTerm.split(',').map(t => t.trim()).filter(t => t);
                if (searchTechs.length > 0 && !searchTechs.some(st => techString.includes(st))) return false;
            }
            return true;
        });
    }

    // --- Sorting Logic (core logic remains the same) ---
    function sortData(data, sortColumn, direction) {
        const propName = sortColumn; // sortColumn will be the actual property name from ProbeResultDisplay

        data.sort((a, b) => {
            let valA = a[propName];
            let valB = b[propName];

            if (Array.isArray(valA)) valA = valA.join(', ');
            if (Array.isArray(valB)) valB = valB.join(', ');

            valA = (valA === undefined || valA === null) ? '' : String(valA);
            valB = (valB === undefined || valB === null) ? '' : String(valB);

            let numA = parseFloat(valA);
            let numB = parseFloat(valB);

            if (propName === 'ContentLength' || propName === 'StatusCode') {
                valA = !isNaN(numA) ? numA : (direction === 'asc' ? Infinity : -Infinity); // Handle non-numeric gracefully for numeric sorts
                valB = !isNaN(numB) ? numB : (direction === 'asc' ? Infinity : -Infinity);
            } else {
                valA = valA.toLowerCase();
                valB = valB.toLowerCase();
            }

            let comparison = 0;
            if (valA > valB) comparison = 1;
            else if (valA < valB) comparison = -1;
            return direction === 'asc' ? comparison : comparison * -1;
        });
        return data;
    }

    // --- Pagination & Rendering ---
    function displayPage(page) {
        currentPage = page;
        const start = (page - 1) * itemsPerPage;
        const end = start + itemsPerPage;
        const paginatedItems = filteredAndSortedData.slice(start, end);

        // Destroy DataTable instance if it exists, so our rendering takes full effect
        if ($.fn.dataTable.isDataTable('#resultsTable')) {
            $('#resultsTable').DataTable().destroy();
            // Ensure tbody is empty for our rendering.
            // renderTableRows also calls $tableBody.empty(), but this is an extra safeguard.
            $('#resultsTable tbody').empty();
        }

        renderTableRows(paginatedItems);
        setupPaginationControls(filteredAndSortedData.length);
        updateResultsCount(filteredAndSortedData.length, allRowsData.length);
    }

    function setupPaginationControls(totalItems) {
        $paginationControls.empty();
        const pageCount = Math.ceil(totalItems / itemsPerPage);
        if (pageCount <= 1) return;

        const $ul = $('<ul></ul>').addClass('pagination pagination-sm');

        const createPageLink = (pageNum, text, isActive, isDisabled) => {
            const $li = $('<li></li>').addClass('page-item');
            if (isActive) $li.addClass('active');
            if (isDisabled) $li.addClass('disabled');

            const $a = $('<a></a>').addClass('page-link').attr('href', '#').text(text || pageNum);
            if (!isDisabled) {
                $a.on('click', (e) => {
                    e.preventDefault();
                    displayPage(pageNum);
                });
            }
            $li.append($a);
            return $li;
        };

        $ul.append(createPageLink(currentPage - 1, 'Previous', false, currentPage === 1));

        let startPage = Math.max(1, currentPage - 2);
        let endPage = Math.min(pageCount, currentPage + 2);
        if (currentPage <= 3) endPage = Math.min(pageCount, 5);
        if (currentPage > pageCount - 3) startPage = Math.max(1, pageCount - 4);

        if (startPage > 1) $ul.append(createPageLink(1, '1'));
        if (startPage > 2) $ul.append($('<li></li>').addClass('page-item disabled').html('<span class="page-link">...</span>'));

        for (let i = startPage; i <= endPage; i++) {
            $ul.append(createPageLink(i, i, i === currentPage));
        }

        if (endPage < pageCount - 1 && endPage + 1 < pageCount) $ul.append($('<li></li>').addClass('page-item disabled').html('<span class="page-link">...</span>'));
        if (endPage < pageCount) $ul.append(createPageLink(pageCount, pageCount));

        $ul.append(createPageLink(currentPage + 1, 'Next', false, currentPage === pageCount));
        $paginationControls.append($ul);
    }

    function updateResultsCount(filteredCount, totalInitialCount) {
        const pageCount = Math.ceil(filteredCount / itemsPerPage);
        let countText = `Showing ${filteredCount} results`;
        if (filteredCount !== totalInitialCount) {
            countText = `Filtered to ${filteredCount} (from ${totalInitialCount}) results`;
        }
        if (filteredCount > 0) {
            countText += `, Page ${currentPage} of ${pageCount || 1}`;
        }
        if (filteredCount === 0 && totalInitialCount > 0) {
            countText = 'No results match filters.';
        }
        $resultsCountInfo.text(countText);
    }

    // --- Main Update Function ---
    function processAndDisplayData() {
        filteredAndSortedData = filterData([...allRowsData]);
        if (currentSortColumn) {
            filteredAndSortedData = sortData(filteredAndSortedData, currentSortColumn, currentSortDirection);
        }
        displayPage(1); // Reset to page 1 after filter/sort change
    }

    // --- Event Listeners ---
    // $globalSearchInput.on('input', function() { currentFilters.globalSearch = $(this).val(); processAndDisplayData(); }); // Global search disabled
    $globalSearchInput.on('input', function () { currentFilters.globalSearch = $(this).val(); processAndDisplayData(); });
    $rootURLFilter.on('change', function () { currentFilters.rootURL = $(this).val(); processAndDisplayData(); });
    $statusCodeFilter.on('change', function () { currentFilters.statusCode = $(this).val(); processAndDisplayData(); });
    $contentTypeFilter.on('change', function () { currentFilters.contentType = $(this).val(); processAndDisplayData(); });
    $techFilterInput.on('input', function () { currentFilters.tech = $(this).val(); processAndDisplayData(); });
    // $targetFilterInput.on('input', function() { currentFilters.target = $(this).val(); processAndDisplayData(); }); // This was likely a typo or old, RootURLFilter is used now
    $urlStatusFilter.on('change', function () { currentFilters.urlStatus = $(this).val(); processAndDisplayData(); });
    $itemsPerPageSelect.on('change', function () {
        itemsPerPage = parseInt($(this).val()) || 10;
        processAndDisplayData();
    });

    $clearAllFiltersBtn.on('click', function () {
        currentFilters = {
            globalSearch: '', // Global search enabled
            rootURL: '',
            statusCode: '',
            contentType: '',
            tech: '',
            urlStatus: ''
        };
        // Reset input field values
        $globalSearchInput.val(''); // Global search enabled
        $rootURLFilter.val('');
        $statusCodeFilter.val('');
        $contentTypeFilter.val('');
        $techFilterInput.val('');
        $urlStatusFilter.val('');

        processAndDisplayData();
    });

    $resultsTable.find('thead th.sortable').on('click', function () {
        const $th = $(this);
        const sortKey = $th.data('sort-key');

        if (!sortKey) return;

        // Removed 'duration' from sortable keys
        const validSortKeys = ['InputURL', 'FinalURL', 'diff_status', 'StatusCode', 'Title', 'ContentType', 'ContentLength'];
        if (!validSortKeys.includes(sortKey)) return;

        if (currentSortColumn === sortKey) {
            currentSortDirection = currentSortDirection === 'asc' ? 'desc' : 'asc';
        } else {
            currentSortColumn = sortKey;
            currentSortDirection = 'asc';
        }

        $resultsTable.find('thead th.sortable').removeClass('sort-asc sort-desc');
        $th.addClass(currentSortDirection === 'asc' ? 'sort-asc' : 'sort-desc');

        filteredAndSortedData = sortData(filteredAndSortedData, currentSortColumn, currentSortDirection);
        displayPage(currentPage);
    });

    // Target List Navigation - REMOVED
    /*
    $('.top-menu').on('click', 'a.nav-link', function(e) { 
            e.preventDefault();
            const $this = $(this);
        currentFilters.rootTarget = $this.data('target'); // This was for the old top-menu
        $('.top-menu a.nav-link').removeClass('active'); 
            $this.addClass('active');
            processAndDisplayData();
        });
    */

    // Details Modal Population
    const $modalDetailsContent = $('#modalDetailsContent');
    const $modalTitle = $('#detailsModal .modal-title');

    $tableBody.on('click', '.view-details-btn', function () {
        const $row = $(this).closest('tr');
        const originalDataIndex = parseInt($row.data('result-index'));
        const resultData = allRowsData[originalDataIndex];

        if (resultData) {
            // Populate the new structured details view
            populateProbeDetails(resultData);
            $modalTitle.text(`Details for: ${resultData.InputURL}`);
        } else {
            $modalTitle.text('Details not found');
            $modalDetailsContent.text('Could not retrieve details for this result.');
        }
    });

    // Helper function to parse headers from text
    function parseHeaders(headersText) {
        if (!headersText || headersText === 'N/A') return {};
        
        try {
            // Try to parse as JSON first
            if (headersText.startsWith('{')) {
                return JSON.parse(headersText);
            }
            
            // If not JSON, try to parse as key-value pairs
            const headers = {};
            const lines = headersText.split('\n');
            lines.forEach(line => {
                const colonIndex = line.indexOf(':');
                if (colonIndex > 0) {
                    const key = line.substring(0, colonIndex).trim();
                    const value = line.substring(colonIndex + 1).trim();
                    headers[key] = value;
                }
            });
            return headers;
        } catch (e) {
            return { 'Raw Headers': headersText };
        }
    }

    // Function to populate probe details in the new structured format
     function populateProbeDetails(resultData) {
         // URL Information
         $('#details-input-url').text(resultData.InputURL || 'N/A');
         $('#details-final-url').text(resultData.FinalURL || 'N/A');
         $('#details-root-target').text(resultData.RootTargetURL || 'N/A');
         
         // Add status badge styling
         const statusBadge = getStatusBadge(resultData.diff_status);
         $('#details-diff-status').html(statusBadge);
         
         $('#details-timestamp').text(resultData.Timestamp || 'N/A');

         // Response Information
         $('#details-method').html(`<span class="badge bg-info">${resultData.Method || 'N/A'}</span>`);
         
         // Add status code badge styling
         const statusCodeBadge = getStatusCodeBadge(resultData.StatusCode);
         $('#details-status-code').html(statusCodeBadge);
         
         $('#details-content-type').text(resultData.ContentType || 'N/A');
         $('#details-content-length').text(formatBytes(resultData.ContentLength) || 'N/A');
         $('#details-web-server').text(resultData.WebServer || 'N/A');

         // Network Information
         $('#details-ips').html(formatIPs((resultData.IPs || []).join(', ')));
         $('#details-cnames').html(formatCNAMEs(resultData.CNAMEs));
         $('#details-asn').text(resultData.ASN && resultData.ASN !== 0 ? resultData.ASN : 'N/A');
         $('#details-asn-org').text(resultData.ASNOrg || 'N/A');

         // Technologies
         $('#details-technologies').html(formatTechnologies((Array.isArray(resultData.Technologies) ? resultData.Technologies.join(', ') : '')));

         // Headers
         populateHeaders(resultData.Headers);

         // Body
         $('#details-body').text(truncateText(resultData.Body, 500) || 'No body content available');
     }

    // Helper function to get status badge
    function getStatusBadge(status) {
        const statusLower = (status || '').toLowerCase();
        let badgeClass = 'bg-secondary';
        
        if (statusLower === 'new') {
            badgeClass = 'bg-success';
        } else if (statusLower === 'old') {
            badgeClass = 'bg-warning';
        } else if (statusLower === 'modified') {
            badgeClass = 'bg-info';
        }
        
        return `<span class="badge ${badgeClass}">${status || 'N/A'}</span>`;
    }

    // Helper function to get status code badge
    function getStatusCodeBadge(statusCode) {
        const code = parseInt(statusCode);
        let badgeClass = 'bg-secondary';
        
        if (code >= 200 && code < 300) {
            badgeClass = 'bg-success';
        } else if (code >= 300 && code < 400) {
            badgeClass = 'bg-info';
        } else if (code >= 400 && code < 500) {
            badgeClass = 'bg-warning';
        } else if (code >= 500) {
            badgeClass = 'bg-danger';
        }
        
        return `<span class="badge ${badgeClass}">${statusCode || 'N/A'}</span>`;
    }

    // Helper function to format bytes
    function formatBytes(bytes) {
        if (!bytes || bytes === '0') return '0 Bytes';
        
        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    // Helper function to format IPs
     function formatIPs(ips) {
         if (!ips || ips === 'N/A') return '<span class="text-muted">N/A</span>';
         
         // If it's a comma-separated list, split and create badges
         const ipList = ips.split(',').map(ip => ip.trim()).filter(ip => ip);
         if (ipList.length > 1) {
             return ipList.map(ip => `<span class="badge bg-light text-dark me-1">${ip}</span>`).join('');
         }
         
         return `<span class="badge bg-light text-dark">${ips}</span>`;
     }

     // Helper function to format CNAMEs
     function formatCNAMEs(cnames) {
         if (!cnames || cnames.length === 0) {
             return '<span class="text-muted">N/A</span>';
         }
         
         if (Array.isArray(cnames)) {
             return cnames.map(cname => `<span class="badge bg-info text-white me-1">${cname}</span>`).join('');
         }
         
         return `<span class="badge bg-info text-white">${cnames}</span>`;
     }

    // Helper function to format technologies
    function formatTechnologies(technologies) {
        if (!technologies || technologies === 'N/A') {
            return '<span class="text-muted">No technologies detected</span>';
        }
        
        // If it's a comma-separated list, split and create badges
        const techList = technologies.split(',').map(tech => tech.trim()).filter(tech => tech);
        if (techList.length > 1) {
            return techList.map(tech => `<span class="badge bg-primary me-1 mb-1">${tech}</span>`).join('');
        }
        
        return `<span class="badge bg-primary">${technologies}</span>`;
    }

    // Helper function to populate headers table
    function populateHeaders(headers) {
        const headersContainer = $('#details-headers');
        headersContainer.empty();
        
        if (!headers || Object.keys(headers).length === 0) {
            headersContainer.append('<tr><td colspan="2" class="text-muted text-center">No headers available</td></tr>');
            return;
        }
        
        // Populate headers table
        Object.entries(headers).forEach(([key, value]) => {
            const row = `
                <tr>
                    <td><strong>${escapeHtml(key)}</strong></td>
                    <td class="text-break">${escapeHtml(value)}</td>
                </tr>
            `;
            headersContainer.append(row);
        });
    }

    // Helper function to escape HTML
    function escapeHtml(text) {
        const map = {
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            '"': '&quot;',
            "'": '&#039;'
        };
        return text.replace(/[&<>"']/g, function(m) { return map[m]; });
    }

    // --- Initial Load ---
    // Initial data (allRowsData) is already populated from window.reportSettings
    // Populate dropdowns from Go template data (UniqueStatusCodes, etc.) is done by Go template itself.
    // This JS assumes those dropdowns are pre-filled.

    // Initialize sorting (e.g., by Input URL asc)
    const $initialSortHeader = $resultsTable.find('thead th[data-sort-key="InputURL"]');
    if ($initialSortHeader.length) {
        currentSortColumn = 'InputURL'; // property name from ProbeResultDisplay
        currentSortDirection = 'asc';
        $initialSortHeader.addClass('sort-asc');
    }

    processAndDisplayData();

    console.log("MonsterInc Report JS (jQuery) Loaded. Initial results: " + allRowsData.length);
});
