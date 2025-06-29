/**
 * Modern Report Dashboard using Alpine.js + Chart.js + AG-Grid
 * Minimal code, maximum functionality
 */

// Ultra-minimal Alpine.js Report App
function reportApp() {
    return {
        loading: true,
        hasData: false,
        showModal: false,
        selectedItem: null,
        gridApi: null,
        stats: {},

        init() {
            const data = window.reportData || [];
            this.hasData = data.length > 0;
            this.stats = this.calculateStats(data);

            setTimeout(() => this.loading = false, 300);
            this.$nextTick(() => {
                this.initGrid();
                this.initCharts();
            });
        },

        calculateStats(data) {
            const total = data.length;
            const success = data.filter(item => {
                const code = parseInt(item.StatusCode);
                return code >= 200 && code < 400;
            }).length;

            const hosts = new Set(data.map(item => {
                const effectiveURL = item.FinalURL || item.InputURL;
                try { return new URL(effectiveURL).hostname; } catch { return ''; }
            }).filter(Boolean));

            const techs = new Set(data.flatMap(item => item.Technologies || []));

            return {
                total,
                successCount: success,
                errorCount: total - success,
                successRate: Math.round((success / total) * 100) || 0,
                errorRate: Math.round(((total - success) / total) * 100) || 0,
                uniqueHosts: hosts.size,
                uniqueTechnologies: techs.size
            };
        },

        initGrid() {
            const grid = document.querySelector('#myGrid');
            if (!grid || !window.agGrid) return;

            const self = this;
            this.gridApi = agGrid.createGrid(grid, {
                columnDefs: [
                    {
                        headerName: 'URL',
                        field: 'effectiveURL',
                        cellRenderer: p => {
                            const effectiveURL = p.data.FinalURL || p.data.InputURL;
                            return `<a href="${effectiveURL}" target="_blank" class="text-blue-600 hover:underline break-words">${effectiveURL}</a>`;
                        },
                        filter: 'agTextColumnFilter',
                        valueGetter: p => p.data.FinalURL || p.data.InputURL,
                        flex: 2
                    },
                    {
                        headerName: 'Status',
                        field: 'diff_status',
                        width: 120,
                        cellRenderer: p => {
                            const colors = { new: 'bg-green-100 text-green-800', existing: 'bg-gray-100 text-gray-800', old: 'bg-red-100 text-red-800' };
                            return `<span class="px-2 py-1 rounded-full text-xs font-medium ${colors[p.value?.toLowerCase()] || colors.existing}">${p.value}</span>`;
                        },
                        filter: 'agSetColumnFilter'
                    },
                    {
                        headerName: 'Code',
                        field: 'StatusCode',
                        width: 100,
                        cellRenderer: p => {
                            const c = parseInt(p.value);
                            let color = 'bg-gray-100 text-gray-800';
                            if (c >= 200 && c < 300) color = 'bg-green-100 text-green-800';
                            else if (c >= 300 && c < 400) color = 'bg-yellow-100 text-yellow-800';
                            else if (c >= 400 && c < 500) color = 'bg-red-100 text-red-800';
                            else if (c >= 500) color = 'bg-purple-100 text-purple-800';
                            return `<span class="px-2 py-1 rounded-full text-xs font-medium ${color}">${p.value}</span>`;
                        },
                        filter: 'agSetColumnFilter'
                    },
                    {
                        headerName: 'Title',
                        field: 'Title',
                        cellRenderer: p => p.value ? `<span class="break-words">${p.value}</span>` : '<span class="text-gray-400">No title</span>',
                        filter: 'agTextColumnFilter',
                        hide: window.innerWidth < 768,
                        flex: 1.5
                    },
                    {
                        headerName: 'Technologies',
                        field: 'Technologies',
                        cellRenderer: p => p.value?.length ? `<div class="flex flex-wrap gap-1 justify-center">${p.value.slice(0, 3).map(t => `<span class="px-1 py-0.5 bg-indigo-100 text-indigo-800 rounded text-xs whitespace-nowrap">${t}</span>`).join('')}${p.value.length > 3 ? `<span class="text-xs text-gray-500">+${p.value.length - 3}</span>` : ''}</div>` : '<span class="text-gray-400 text-xs">None</span>',
                        filter: 'agSetColumnFilter',
                        hide: window.innerWidth < 1024,
                        flex: 1.2
                    },
                    {
                        headerName: 'Details',
                        field: 'details',
                        width: 100,
                        cellRenderer: p => `<button onclick="openRowDetails(${p.node.rowIndex})" class="px-2 py-1 bg-blue-600 hover:bg-blue-700 text-white rounded text-xs">View</button>`,
                        sortable: false,
                        filter: false
                    }
                ],
                rowData: window.reportData || [],
                defaultColDef: {
                    sortable: true,
                    filter: true,
                    resizable: true,
                    floatingFilter: true,
                    flex: 1,
                    cellStyle: { textAlign: 'center' },
                    autoHeight: true,
                    wrapText: true
                },
                pagination: true,
                paginationPageSize: 25,
                paginationPageSizeSelector: [10, 25, 50, 100],
                headerHeight: 52,
                onGridReady: params => {
                    this.gridApi = params.api;
                    params.api.sizeColumnsToFit();
                    window.addEventListener('resize', () => setTimeout(() => params.api.sizeColumnsToFit(), 100));
                }
            }).api;
        },

        initCharts() {
            if (!window.Chart) return;

            const data = window.reportData || [];

            // Status chart
            const statusCtx = document.getElementById('statusChart');
            if (statusCtx) {
                const statusCounts = {};
                data.forEach(item => {
                    const c = parseInt(item.StatusCode);
                    const range = c >= 200 && c < 300 ? '2xx Success' : c >= 300 && c < 400 ? '3xx Redirect' : c >= 400 && c < 500 ? '4xx Client Error' : c >= 500 ? '5xx Server Error' : 'Other';
                    statusCounts[range] = (statusCounts[range] || 0) + 1;
                });

                new Chart(statusCtx, {
                    type: 'doughnut',
                    data: {
                        labels: Object.keys(statusCounts),
                        datasets: [{ data: Object.values(statusCounts), backgroundColor: ['#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#6b7280'], borderWidth: 2, borderColor: '#ffffff' }]
                    },
                    options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { position: 'bottom', labels: { padding: 20, usePointStyle: true, font: { size: 12 } } } } }
                });
            }

            // Diff chart  
            const diffCtx = document.getElementById('diffChart');
            if (diffCtx) {
                const diffCounts = {};
                data.forEach(item => diffCounts[item.diff_status || 'unknown'] = (diffCounts[item.diff_status || 'unknown'] || 0) + 1);

                new Chart(diffCtx, {
                    type: 'bar',
                    data: {
                        labels: Object.keys(diffCounts),
                        datasets: [{ data: Object.values(diffCounts), backgroundColor: ['#10b981', '#6b7280', '#ef4444', '#3b82f6'], borderWidth: 0, borderRadius: 8 }]
                    },
                    options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false } }, scales: { y: { beginAtZero: true, grid: { color: '#f3f4f6' } }, x: { grid: { display: false } } } }
                });
            }
        },

        openModal(rowIndex) {
            this.selectedItem = (window.reportData || [])[rowIndex];
            this.showModal = true;
        },

        closeModal() {
            this.showModal = false;
            this.selectedItem = null;
        },

        exportData(format) {
            if (!this.gridApi) return;
            const fileName = `monsterinc-scan-${new Date().toISOString().split('T')[0]}`;
            if (format === 'csv') this.gridApi.exportDataAsCsv({ fileName: fileName + '.csv' });
            else if (format === 'excel') this.gridApi.exportDataAsExcel({ fileName: fileName + '.xlsx' });
        },

        formatBytes(bytes) {
            if (!bytes) return '0 Bytes';
            const k = 1024, sizes = ['Bytes', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }
    }
}

// Global function to handle grid button clicks
window.openRowDetails = function (rowIndex) {
    const appElement = document.querySelector('[x-data]');
    if (appElement && appElement._x_dataStack && appElement._x_dataStack[0]) {
        appElement._x_dataStack[0].openModal(rowIndex);
    }
};

// Log when ready
document.addEventListener('alpine:init', () => {
    console.log('🚀 MonsterInc Report Dashboard ready');
});