// Custom JavaScript for MonsterInc HTML Report

document.addEventListener('DOMContentLoaded', function () {
    const resultsTable = document.getElementById('resultsTable');
    const tableBody = resultsTable.querySelector('tbody');
    // allRows will be populated from window.reportSettings.initialProbeResults
    let allRowsData = []; 
    if (typeof window.reportSettings !== 'undefined' && window.reportSettings.initialProbeResults) {
        allRowsData = window.reportSettings.initialProbeResults;
    }

    const globalSearchInput = document.getElementById('globalSearch');
    const statusCodeFilter = document.getElementById('statusCodeFilter');
    const contentTypeFilter = document.getElementById('contentTypeFilter');
    const techFilterInput = document.getElementById('techFilter');
    const paginationControls = document.getElementById('pagination-controls');
    const targetList = document.getElementById('target-list');
    const currentFilterSummaryEl = document.getElementById('currentFilterSummary');

    const itemsPerPage = (typeof window.reportSettings !== 'undefined' && window.reportSettings.itemsPerPage) ? window.reportSettings.itemsPerPage : 25;
    let currentPage = 1;
    let currentSortColumn = null; 
    let currentSortDirection = 'asc';
    let currentFilters = {
        global: '',
        statusCode: '',
        contentType: '',
        tech: '',
        rootTarget: 'all'
    };

    // --- Helper: Truncate text ---
    function truncateText(text, maxLength) {
        if (text && text.length > maxLength) {
            return text.substring(0, maxLength - 3) + "...";
        }
        return text || ''; // Ensure not null/undefined
    }

    // --- Render Table Rows from Data ---
    function renderTableRows(dataToRender) {
        tableBody.innerHTML = ''; // Clear existing rows
        if (!dataToRender || dataToRender.length === 0) {
            const colCount = resultsTable.querySelector('thead th') ? resultsTable.querySelectorAll('thead th').length : 9;
            tableBody.innerHTML = `<tr><td colspan="${colCount}" class="text-center">No results match your filters.</td></tr>`;
            return;
        }

        dataToRender.forEach(pr => {
            const row = tableBody.insertRow();
            row.dataset.rootTarget = pr.RootTargetURL || '';

            // Corresponds to headers: "Input URL", "Final URL", "Status Code", "Content Length", "Content Type", "Title", "Web Server", "Technologies", "IPs"
            let cell = row.insertCell(); cell.textContent = truncateText(pr.InputURL, 50); cell.title = pr.InputURL;
            cell = row.insertCell(); 
            if (pr.FinalURL) {
                const finalLink = document.createElement('a');
                finalLink.href = pr.FinalURL;
                finalLink.textContent = truncateText(pr.FinalURL, 50);
                finalLink.target = '_blank';
                cell.appendChild(finalLink);
                cell.title = pr.FinalURL;
            } else {
                cell.textContent = '-';
            }
            cell = row.insertCell(); cell.textContent = pr.StatusCode || '-'; cell.classList.add(`status-code-${pr.StatusCode}`);
            cell = row.insertCell(); cell.textContent = pr.ContentLength !== undefined ? pr.ContentLength : '-';
            cell = row.insertCell(); cell.textContent = truncateText(pr.ContentType, 30); cell.title = pr.ContentType;
            cell = row.insertCell(); cell.textContent = truncateText(pr.Title, 70); cell.title = pr.Title;
            cell = row.insertCell(); cell.textContent = truncateText(pr.WebServer, 30); cell.title = pr.WebServer;
            cell = row.insertCell(); 
            const techString = Array.isArray(pr.Technologies) ? pr.Technologies.join(', ') : '';
            cell.textContent = truncateText(techString, 40); cell.title = techString;
            cell = row.insertCell(); cell.textContent = Array.isArray(pr.IPs) ? pr.IPs.join(', ') : '-'; cell.title = Array.isArray(pr.IPs) ? pr.IPs.join(', ') : '-';
        });
    }

    // --- Populate Filters ---
    function populateDropdownFilters() {
        const statusCodes = new Set();
        const contentTypes = new Set();
        const rootTargetsFromData = new Set();

        allRowsData.forEach(pr => {
            if (pr.StatusCode) statusCodes.add(pr.StatusCode.toString());
            if (pr.ContentType) contentTypes.add(pr.ContentType.split(';')[0].trim()); // Get primary content type
            if (pr.RootTargetURL) rootTargetsFromData.add(pr.RootTargetURL);
        });

        // Populate Status Code Filter
        Array.from(statusCodes).sort().forEach(code => {
            const option = document.createElement('option');
            option.value = code; option.textContent = code;
            statusCodeFilter.appendChild(option);
        });

        // Populate Content Type Filter
        Array.from(contentTypes).sort().forEach(type => {
            if (type) { // Ensure type is not empty
                const option = document.createElement('option');
                option.value = type; option.textContent = type;
                contentTypeFilter.appendChild(option);
            }
        });
        
        // Populate Target List (if not already done by Go template, or to add counts)
        const targetListUl = document.getElementById('target-list');
        targetListUl.innerHTML = '<li class="nav-item"><a class="nav-link active" href="#" data-target="all">All Targets (' + allRowsData.length + ')</a></li>'; // Reset with total
        Array.from(rootTargetsFromData).sort().forEach(target => {
            const count = allRowsData.filter(r => r.RootTargetURL === target).length;
            const li = document.createElement('li');
            li.classList.add('nav-item');
            const a = document.createElement('a');
            a.classList.add('nav-link');
            a.href = `#${target}`;
            a.dataset.target = target;
            a.textContent = `${target} (${count})`;
            targetListUl.appendChild(li);
        });
    }

    // --- Filtering Logic ---
    function filterData(data) {
        const globalSearchTerm = currentFilters.global.toLowerCase();
        const statusCode = currentFilters.statusCode;
        const contentType = currentFilters.contentType.toLowerCase();
        const techTerm = currentFilters.tech.toLowerCase();
        const rootTarget = currentFilters.rootTarget;

        let summaryParts = [];
        if (rootTarget !== 'all') summaryParts.push(`Target: ${rootTarget}`);
        if (globalSearchTerm) summaryParts.push(`Search: "${globalSearchTerm}"`);
        if (statusCode) summaryParts.push(`Status: ${statusCode}`);
        if (contentType) summaryParts.push(`Type: ${contentType}`);
        if (techTerm) summaryParts.push(`Tech: "${techTerm}"`);
        currentFilterSummaryEl.textContent = summaryParts.length > 0 ? `(${summaryParts.join(", ")})` : '';

        return data.filter(pr => {
            if (rootTarget !== 'all' && pr.RootTargetURL !== rootTarget) return false;

            let matchesGlobal = true;
            if (globalSearchTerm) {
                matchesGlobal = (
                    (pr.InputURL && pr.InputURL.toLowerCase().includes(globalSearchTerm)) ||
                    (pr.FinalURL && pr.FinalURL.toLowerCase().includes(globalSearchTerm)) ||
                    (pr.Title && pr.Title.toLowerCase().includes(globalSearchTerm)) ||
                    (pr.WebServer && pr.WebServer.toLowerCase().includes(globalSearchTerm)) ||
                    (Array.isArray(pr.Technologies) && pr.Technologies.join(', ').toLowerCase().includes(globalSearchTerm)) ||
                    (Array.isArray(pr.IPs) && pr.IPs.join(', ').toLowerCase().includes(globalSearchTerm))
                );
            }
            if (!matchesGlobal) return false;

            if (statusCode && (!pr.StatusCode || pr.StatusCode.toString() !== statusCode)) return false;
            if (contentType && (!pr.ContentType || !pr.ContentType.toLowerCase().includes(contentType))) return false;

            if (techTerm) {
                const techString = Array.isArray(pr.Technologies) ? pr.Technologies.join(', ').toLowerCase() : "";
                const searchTechs = techTerm.split(',').map(t => t.trim()).filter(t => t);
                if (searchTechs.length > 0 && !searchTechs.some(st => techString.includes(st))) return false;
            }
            return true;
        });
    }

    // --- Sorting Logic ---
    function sortData(data, sortColumn, direction) {
        const sortPropMap = {
            'input-url': 'InputURL',
            'final-url': 'FinalURL',
            'status-code': 'StatusCode',
            'content-length': 'ContentLength',
            'content-type': 'ContentType',
            'title': 'Title',
            'web-server': 'WebServer',
            'technologies': 'Technologies',
            'ips': 'IPs'
        };
        const propName = sortPropMap[sortColumn]; 
        if (!propName) return data;

        data.sort((a, b) => {
            let valA = a[propName];
            let valB = b[propName];

            // Handle array types for sorting (e.g., Technologies, IPs - sort by first element or count)
            if (Array.isArray(valA)) valA = valA.join(', ');
            if (Array.isArray(valB)) valB = valB.join(', ');

            valA = (valA === undefined || valA === null) ? '' : String(valA).toLowerCase();
            valB = (valB === undefined || valB === null) ? '' : String(valB).toLowerCase();

            const numA = parseFloat(valA);
            const numB = parseFloat(valB);
            if (propName === 'ContentLength' || propName === 'StatusCode') {
                 if (!isNaN(numA) && !isNaN(numB)) {
                    valA = numA; valB = numB;
                }
            }

            let comparison = 0;
            if (valA > valB) comparison = 1;
            else if (valA < valB) comparison = -1;
            return direction === 'asc' ? comparison : comparison * -1;
        });
        return data;
    }

    // --- Pagination Logic ---
    function displayPage(processedData, page) {
        currentPage = page;
        const start = (page - 1) * itemsPerPage;
        const end = start + itemsPerPage;
        const paginatedItems = processedData.slice(start, end);
        
        renderTableRows(paginatedItems);
        setupPaginationControls(processedData.length);
    }
    
    // (setupPaginationControls remains largely the same, ensure it is called with total filtered items)
    function setupPaginationControls(totalItems) {
        paginationControls.innerHTML = '';
        const pageCount = Math.ceil(totalItems / itemsPerPage);
        if (pageCount <= 1) return;

        const createPageLink = (pageNum, text, isActive, isDisabled) => {
            const li = document.createElement('li');
            li.classList.add('page-item');
            if (isActive) li.classList.add('active');
            if (isDisabled) li.classList.add('disabled');
            const a = document.createElement('a');
            a.classList.add('page-link');
            a.href = '#';
            a.textContent = text || pageNum;
            if (!isDisabled) {
                a.addEventListener('click', (e) => { 
                    e.preventDefault(); 
                    displayPage(filterAndSortCurrentData(), pageNum);
                });
            }
            li.appendChild(a);
            return li;
        };

        paginationControls.appendChild(createPageLink(currentPage - 1, 'Previous', false, currentPage === 1));

        let startPage = Math.max(1, currentPage - 2);
        let endPage = Math.min(pageCount, currentPage + 2);
        if (currentPage <= 3) endPage = Math.min(pageCount, 5);
        if (currentPage > pageCount - 3) startPage = Math.max(1, pageCount - 4);

        if (startPage > 1) paginationControls.appendChild(createPageLink(1, '1'));
        if (startPage > 2) paginationControls.appendChild(createPageLink(0, '...', false, true));

        for (let i = startPage; i <= endPage; i++) {
            paginationControls.appendChild(createPageLink(i, i, i === currentPage));
        }

        if (endPage < pageCount) paginationControls.appendChild(createPageLink(0, '...', false, true));
        if (endPage < pageCount -1) paginationControls.appendChild(createPageLink(pageCount, pageCount));
        
        paginationControls.appendChild(createPageLink(currentPage + 1, 'Next', false, currentPage === pageCount));
    }


    // --- Main Update Function ---
    function filterAndSortCurrentData() {
        let processedData = filterData([...allRowsData]); // Use a copy of allRowsData
        if (currentSortColumn) {
            processedData = sortData(processedData, currentSortColumn, currentSortDirection);
        }
        return processedData;
    }
    
    function updateTable() {
        const processedData = filterAndSortCurrentData();
        displayPage(processedData, 1); 
    }

    // --- Event Listeners ---
    globalSearchInput.addEventListener('input', () => { currentFilters.global = globalSearchInput.value; updateTable(); });
    statusCodeFilter.addEventListener('change', () => { currentFilters.statusCode = statusCodeFilter.value; updateTable(); });
    contentTypeFilter.addEventListener('change', () => { currentFilters.contentType = contentTypeFilter.value; updateTable(); });
    techFilterInput.addEventListener('input', () => { currentFilters.tech = techFilterInput.value; updateTable(); });

    resultsTable.querySelectorAll('thead th.sortable').forEach(th => {
        th.addEventListener('click', () => {
            const columnName = th.dataset.columnName;
            if (currentSortColumn === columnName) {
                currentSortDirection = currentSortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                currentSortColumn = columnName;
                currentSortDirection = 'asc';
            }
            resultsTable.querySelectorAll('thead th.sortable').forEach(header => {
                header.classList.remove('sort-asc', 'sort-desc');
                if (header.dataset.columnName === currentSortColumn) {
                    header.classList.add(currentSortDirection === 'asc' ? 'sort-asc' : 'sort-desc');
                }
            });
            // Re-filter and sort, then display current page
            const processedData = filterAndSortCurrentData();
            displayPage(processedData, currentPage); // Stay on current page for sorting
        });
    });

    if (targetList) {
        targetList.addEventListener('click', (event) => {
            const anchor = event.target.closest('a.nav-link');
            if (anchor) {
                event.preventDefault();
                currentFilters.rootTarget = anchor.dataset.target;
                targetList.querySelectorAll('.nav-link').forEach(link => link.classList.remove('active'));
                anchor.classList.add('active');
                updateTable();
            }
        });
    }
    
    // Initialize Bootstrap Tooltips
    if (typeof bootstrap !== 'undefined' && typeof bootstrap.Tooltip === 'function') {
        const tooltipTriggerList = [].slice.call(document.querySelectorAll('[title]'));
        tooltipTriggerList.forEach(function (tooltipTriggerEl) {
            if (tooltipTriggerEl.getAttribute('title')) { // Only init if title is not empty
                 new bootstrap.Tooltip(tooltipTriggerEl, { trigger: 'hover' });
            }
        });
    }

    // --- Initial Load ---
    populateDropdownFilters();
    const firstSortableHeader = resultsTable.querySelector('thead th.sortable[data-column-name="input-url"]');
    if (firstSortableHeader) {
        currentSortColumn = firstSortableHeader.dataset.columnName;
        currentSortDirection = 'asc';
        firstSortableHeader.classList.add('sort-asc');
    }
    updateTable(); 

    console.log("MonsterInc Report JS Loaded. Total initial results:", allRowsData.length);
});

// Dummy ReportData for environments where Go template doesn't inject it (e.g. static serving for dev)
if (typeof ReportData === 'undefined') {
    var ReportData = {
        itemsPerPage: 25 // Default if not injected by Go template
    };
} 