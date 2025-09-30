class WebAnalyzer {
    constructor() {
        this.apiBaseUrl = 'https://api.web-analyzer.dev/v1';
        this.authToken = 'v4.public.eyJhdWQiOiJ3ZWItYW5hbHl6ZXItYXBpIiwiZXhwIjoiMjA2My0wOS0xOFQwMjoyMDoxNyswMjowMCIsImlhdCI6IjIwMjUtMDktMjdUMDI6MjA6MTcrMDI6MDAiLCJpc3MiOiJ3ZWItYW5hbHl6ZXItc2VydmljZSIsImp0aSI6InByb3Blci1wYXNldG8tdjQtdG9rZW4iLCJuYmYiOiIyMDI1LTA5LTI3VDAyOjIwOjE3KzAyOjAwIiwic2NvcGVzIjpbImFuYWx5emUiLCJyZWFkIl0sInN1YiI6InRlc3QtdXNlciJ9MVH2eMTu9jMw6ZUIB538m-4gUoonWUbkHPDReqzD_2lojhtO2d1l3FXc6RCOozfW3fIdbU9y9SWAzBBamKydAQ';
        this.currentAnalysisId = null;
        this.currentUrl = null;
        this.eventSource = null;
        this.intentionallyClosed = false;

        this.initializeElements();
        this.attachEventListeners();
        this.initializeKonamiCode();
    }

    initializeElements() {
        this.form = document.getElementById('analyzeForm');
        this.urlInput = document.getElementById('urlInput');
        this.analyzeButton = document.getElementById('analyzeButton');
        this.buttonText = this.analyzeButton.querySelector('.button-text');
        this.spinner = this.analyzeButton.querySelector('.spinner');
        this.results = document.getElementById('results');
        this.analysisInfo = document.getElementById('analysisInfo');
        this.progressContainer = document.getElementById('progressContainer');
        this.progressFill = document.getElementById('progressFill');
        this.progressText = document.getElementById('progressText');
        this.progressLog = document.getElementById('progressLog');
        this.analysisResults = document.getElementById('analysisResults');
        this.error = document.getElementById('error');
        this.errorMessage = document.getElementById('errorMessage');
    }

    attachEventListeners() {
        this.form.addEventListener('submit', (e) => this.handleSubmit(e));
    }

    async handleSubmit(event) {
        event.preventDefault();

        const url = this.urlInput.value.trim();
        if (!url) {
            this.showError('Please enter a valid URL');
            return;
        }

        this.clearResults();
        this.setLoading(true);

        try {
            console.log('Submitting analysis for URL:', url);
            const analysisId = await this.submitAnalysis(url);
            console.log('Analysis submitted with ID:', analysisId);

            this.currentAnalysisId = analysisId;
            this.currentUrl = url;
            this.showResults(url, analysisId);

            console.log('About to start progress monitoring...');
            this.startProgressMonitoring(analysisId);
        } catch (error) {
            console.error('Error in handleSubmit:', error);
            this.showError(`Failed to submit analysis: ${error.message}`);
            this.setLoading(false);
        }
    }

    async submitAnalysis(url) {
        const response = await fetch(`${this.apiBaseUrl}/analyze`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${this.authToken}`
            },
            body: JSON.stringify({ url })
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.message || `HTTP ${response.status}: ${response.statusText}`);
        }

        const data = await response.json();
        return data.analysis_id;
    }

    async getAnalysisResult(analysisId) {
        const response = await fetch(`${this.apiBaseUrl}/analysis/${analysisId}`, {
            headers: {
                'Authorization': `Bearer ${this.authToken}`
            }
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.message || `HTTP ${response.status}: ${response.statusText}`);
        }

        return await response.json();
    }

    startProgressMonitoring(analysisId) {
        console.log('=== Starting Progress Monitoring ===');
        console.log('Analysis ID:', analysisId);

        this.progressContainer.classList.remove('hidden');
        this.progressLog.innerHTML = '';
        this.intentionallyClosed = false;

        // Close existing EventSource if any
        if (this.eventSource) {
            console.log('Closing existing EventSource');
            this.intentionallyClosed = true;
            this.eventSource.close();
            this.intentionallyClosed = false;
        }

        // EventSource doesn't support custom headers, so we pass the token as a query parameter
        const sseUrl = `${this.apiBaseUrl}/analysis/${analysisId}/events?token=${encodeURIComponent(this.authToken)}`;
        console.log('Connecting to SSE URL:', sseUrl);

        try {
            this.eventSource = new EventSource(sseUrl);
            console.log('EventSource created, readyState:', this.eventSource.readyState);
        } catch (error) {
            console.error('Error creating EventSource:', error);
            return;
        }

        this.eventSource.onopen = () => {
            console.log('SSE connection opened successfully');
        };

        // Add a general message handler to catch any messages
        this.eventSource.onmessage = (event) => {
            console.debug('Received general message:', event);
            console.debug('Message data:', event.data);
        };

        this.eventSource.addEventListener('analysis_started', (event) => {
            console.log('Received analysis_started event:', event);
            console.log('Event type:', event.constructor.name);
            console.log('Event data:', event.data);
            try {
                if (event.data) {
                    const analysis = JSON.parse(event.data);
                    this.updateProgressFromAnalysis(analysis);
                } else {
                    console.error('No data in analysis_started event');
                }
            } catch (e) {
                console.error('Error parsing analysis_started event data:', e, event);
            }
        });

        this.eventSource.addEventListener('analysis_progress', (event) => {
            try {
                const analysis = JSON.parse(event.data);
                this.updateProgressFromAnalysis(analysis);
            } catch (e) {
                console.error('Error parsing analysis_progress event data:', e, event);
            }
        });

        this.eventSource.addEventListener('analysis_completed', (event) => {
            console.log('Received analysis_completed event');
            // Set intentionallyClosed FIRST to prevent onerror from triggering fallback
            this.intentionallyClosed = true;

            try {
                const analysis = JSON.parse(event.data);
                this.updateProgressFromAnalysis(analysis);
                // Use the analysis data from the event instead of fetching again
                this.displayAnalysisResult(analysis);
                this.celebrate();
                this.eventSource.close();
                this.setLoading(false);
            } catch (e) {
                console.error('Error parsing analysis_completed event data:', e, event);
                this.eventSource.close();
                this.setLoading(false);
            }
        });

        this.eventSource.addEventListener('analysis_failed', (event) => {
            console.log('Received analysis_failed event');
            // Set intentionallyClosed FIRST to prevent onerror from triggering fallback
            this.intentionallyClosed = true;

            try {
                const analysis = JSON.parse(event.data);
                const errorMessage = analysis.error ? analysis.error.message : 'Analysis failed';
                this.showError(`Analysis failed: ${errorMessage}`);
                this.eventSource.close();
                this.setLoading(false);
            } catch (e) {
                console.error('Error parsing analysis_failed event data:', e, event);
                this.showError('Analysis failed with unknown error');
                this.eventSource.close();
                this.setLoading(false);
            }
        });

        this.eventSource.addEventListener('heartbeat', (event) => {

        })

        // Handle connection errors (different from analysis_failed)
        this.eventSource.addEventListener('error', (event) => {
            console.log('Received error event');
            // Set intentionallyClosed FIRST to prevent onerror from triggering fallback
            this.intentionallyClosed = true;

            try {
                console.debug('Error event data:', event.data);
                const errorData = JSON.parse(event.data);
                this.showError(`Connection error: ${errorData.error}`);
                this.eventSource.close();
                this.setLoading(false);
            } catch (e) {
                console.error('Error parsing error event data:', e, event);
                this.showError('Connection error occurred');
                this.eventSource.close();
                this.setLoading(false);
            }
        });

        this.eventSource.onerror = (error) => {
            console.error('=== SSE Error ===');
            console.error('Error event:', error);
            console.error('EventSource readyState:', this.eventSource.readyState);
            console.error('EventSource URL:', this.eventSource.url);

            // ReadyState meanings: 0=CONNECTING, 1=OPEN, 2=CLOSED
            const stateNames = ['CONNECTING', 'OPEN', 'CLOSED'];
            console.error('EventSource state:', stateNames[this.eventSource.readyState] || 'UNKNOWN');

            // Only fallback to polling if the connection wasn't intentionally closed
            if (!this.intentionallyClosed) {
                this.fallbackToPolling(analysisId);
            }
        };
    }

    updateProgressFromAnalysis(analysis) {
        // Transform Analysis domain object to display format
        const statusMap = {
            'requested': { progress: 10, stage: 'starting', message: 'Starting analysis...' },
            'in_progress': { progress: 50, stage: 'analyzing', message: 'Analyzing web page...' },
            'completed': { progress: 100, stage: 'completed', message: 'Analysis completed successfully' },
            'failed': { progress: 100, stage: 'failed', message: 'Analysis failed' }
        };

        const statusInfo = statusMap[analysis.status] || { progress: 0, stage: 'unknown', message: 'Processing...' };

        // Use analysis timestamps or current time
        const timestamp = analysis.created_at ? new Date(analysis.created_at).toLocaleTimeString() : new Date().toLocaleTimeString();

        this.updateProgress({
            progress: statusInfo.progress,
            stage: statusInfo.stage,
            message: statusInfo.message,
            timestamp: timestamp
        });
    }

    updateProgress(data) {
        const progress = data.progress || 0;
        const stage = data.stage || 'unknown';
        const message = data.message || 'Processing...';
        const timestamp = data.timestamp || new Date().toLocaleTimeString();

        // Update progress bar
        this.progressFill.style.width = `${progress}%`;
        this.progressText.textContent = `${progress}% - ${message}`;

        // Add to progress log
        const logEntry = document.createElement('div');
        logEntry.className = 'progress-entry';
        logEntry.innerHTML = `
            <span class="timestamp">[${timestamp}]</span>
            <span class="stage">${stage}</span>
            <span class="message">${message}</span>
        `;
        this.progressLog.appendChild(logEntry);

        // Auto-scroll to bottom
        this.progressLog.scrollTop = this.progressLog.scrollHeight;
    }

    async fallbackToPolling(analysisId) {
        console.log('Falling back to polling...');

        const pollInterval = setInterval(async () => {
            try {
                const result = await this.getAnalysisResult(analysisId);

                if (result.status === 'completed') {
                    clearInterval(pollInterval);
                    this.displayAnalysisResult(result);
                    this.setLoading(false);
                } else if (result.status === 'failed') {
                    clearInterval(pollInterval);
                    this.showError('Analysis failed');
                    this.setLoading(false);
                }
            } catch (error) {
                console.error('Polling error:', error);
                clearInterval(pollInterval);
                this.setLoading(false);
            }
        }, 2000);

        // Stop polling after 5 minutes
        setTimeout(() => {
            clearInterval(pollInterval);
            this.setLoading(false);
        }, 300000);
    }

    displayAnalysisResult(result) {
        this.analysisResults.classList.remove('hidden');

        let durationText = 'N/A';
        if (result.duration) {
            const seconds = result.duration / 1000000000;
            durationText = seconds < 1 ? `${Math.round(seconds * 1000)}ms` : `${seconds.toFixed(2)}s`;
        }

        let html = `
            <h3>📊 Analysis Complete</h3>

            <!-- Summary Cards -->
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-icon">⏱️</div>
                    <div class="stat-value">${durationText}</div>
                    <div class="stat-label">Duration</div>
                </div>
        `;

        let statData = {};

        if (result.results) {
            const data = result.results;
            const totalHeadings = (data.heading_counts?.h1 || 0) + (data.heading_counts?.h2 || 0) +
                                 (data.heading_counts?.h3 || 0) + (data.heading_counts?.h4 || 0) +
                                 (data.heading_counts?.h5 || 0) + (data.heading_counts?.h6 || 0);

            statData = {
                headings: totalHeadings,
                links: data.links?.total_count || 0,
                forms: data.forms?.total_count || 0
            };

            html += `
                <div class="stat-card" onclick="document.getElementById('headings-section')?.scrollIntoView({ behavior: 'smooth' })" style="cursor: pointer;">
                    <div class="stat-icon">📑</div>
                    <div class="stat-value" data-counter="${totalHeadings}">0</div>
                    <div class="stat-label">Headings</div>
                </div>
                <div class="stat-card" onclick="document.getElementById('links-section')?.scrollIntoView({ behavior: 'smooth' })" style="cursor: pointer;">
                    <div class="stat-icon">🔗</div>
                    <div class="stat-value" data-counter="${data.links?.total_count || 0}">0</div>
                    <div class="stat-label">Total Links</div>
                </div>
                <div class="stat-card" onclick="document.getElementById('forms-section')?.scrollIntoView({ behavior: 'smooth' })" style="cursor: pointer;">
                    <div class="stat-icon">📝</div>
                    <div class="stat-value" data-counter="${data.forms?.total_count || 0}">0</div>
                    <div class="stat-label">Forms</div>
                </div>
            `;
        }

        html += `</div>`;

        // Basic Information
        html += `
            <div class="result-section">
                <h4>📋 Basic Information</h4>
                <div class="info-grid">
                    <div class="info-item">
                        <span class="info-label">URL:</span>
                        <a href="${this.currentUrl}" target="_blank" rel="noopener" class="info-value link">${this.currentUrl}</a>
                    </div>
                    <div class="info-item">
                        <span class="info-label">Status:</span>
                        <span class="info-value status-badge status-${result.status}">${result.status}</span>
                    </div>
                    <div class="info-item">
                        <span class="info-label">Created:</span>
                        <span class="info-value">${new Date(result.created_at).toLocaleString()}</span>
                    </div>
        `;

        if (result.completed_at) {
            html += `
                    <div class="info-item">
                        <span class="info-label">Completed:</span>
                        <span class="info-value">${new Date(result.completed_at).toLocaleString()}</span>
                    </div>
            `;
        }

        if (result.content_size) {
            const sizeFormatted = this.formatBytes(result.content_size);
            html += `
                    <div class="info-item">
                        <span class="info-label">Content Size:</span>
                        <span class="info-value">${sizeFormatted}</span>
                    </div>
            `;
        }

        if (result.content_hash) {
            html += `
                    <div class="info-item">
                        <span class="info-label">Content Hash:</span>
                        <span class="info-value" style="font-family: monospace; font-size: 0.85em;">${result.content_hash}</span>
                    </div>
            `;
        }

        html += `</div></div>`;

        if (result.results) {
            const data = result.results;

            // Page Information
            html += `
                <div class="result-section">
                    <h4>🌐 Page Information</h4>
                    <div class="info-grid">
                        <div class="info-item">
                            <span class="info-label">HTML Version:</span>
                            <span class="info-value">${data.html_version || 'N/A'}</span>
                        </div>
                        <div class="info-item">
                            <span class="info-label">Title:</span>
                            <span class="info-value">${this.escapeHtml(data.title) || 'N/A'}</span>
                        </div>
                    </div>
                </div>
            `;

            // Headings Analysis
            if (data.heading_counts) {
                const counts = data.heading_counts;
                const totalHeadings = counts.h1 + counts.h2 + counts.h3 + counts.h4 + counts.h5 + counts.h6;

                if (totalHeadings > 0) {
                    const headingData = ['h1', 'h2', 'h3', 'h4', 'h5', 'h6']
                        .map(level => ({ level, count: counts[level] || 0 }))
                        .filter(item => item.count > 0);

                    html += `
                        <div class="result-section" id="headings-section">
                            <h4>📑 Headings Distribution</h4>
                            <div class="pie-chart-container">
                                <div id="pie-chart"></div>
                            </div>
                        </div>
                    `;

                    // Defer D3 chart creation until after DOM update
                    setTimeout(() => this.createPieChart(headingData, totalHeadings), 100);
                }
            }

            // Links Analysis
            if (data.links) {
                const links = data.links;
                html += `
                    <div class="result-section" id="links-section">
                        <h4>🔗 Links Analysis</h4>
                        <div class="links-summary">
                            <div class="link-type-card">
                                <div class="link-type-icon total">📊</div>
                                <div class="link-type-count">${links.total_count || 0}</div>
                                <div class="link-type-label">Total</div>
                            </div>
                            <div class="link-type-card">
                                <div class="link-type-icon internal">🏠</div>
                                <div class="link-type-count">${links.internal_count || 0}</div>
                                <div class="link-type-label">Internal</div>
                            </div>
                            <div class="link-type-card">
                                <div class="link-type-icon external">🌍</div>
                                <div class="link-type-count">${links.external_count || 0}</div>
                                <div class="link-type-label">External</div>
                            </div>
                        </div>
                `;

                // Inaccessible Links
                if (links.inaccessible_links && links.inaccessible_links.length > 0) {
                    html += `
                        <div class="inaccessible-links">
                            <h5>⚠️ Inaccessible Links (${links.inaccessible_links.length})</h5>
                            <div class="broken-links-list">
                    `;

                    links.inaccessible_links.forEach(link => {
                        html += `
                            <div class="broken-link-item">
                                <div class="broken-link-status">${link.status_code}</div>
                                <div class="broken-link-details">
                                    <div class="broken-link-url">${this.escapeHtml(link.url)}</div>
                                    <div class="broken-link-error">${this.escapeHtml(link.error)}</div>
                                </div>
                            </div>
                        `;
                    });

                    html += `</div></div>`;
                }

                html += `</div>`;
            }

            // Forms Analysis
            if (data.forms && data.forms.total_count > 0) {
                const forms = data.forms;
                html += `
                    <div class="result-section" id="forms-section">
                        <h4>📝 Forms Analysis</h4>
                        <div class="info-grid">
                            <div class="info-item">
                                <span class="info-label">Total Forms:</span>
                                <span class="info-value">${forms.total_count}</span>
                            </div>
                            <div class="info-item">
                                <span class="info-label">Login Forms Detected:</span>
                                <span class="info-value ${forms.login_forms_detected > 0 ? 'warning' : ''}">${forms.login_forms_detected}</span>
                            </div>
                        </div>
                `;

                if (forms.login_form_details && forms.login_form_details.length > 0) {
                    html += `
                        <div class="login-forms-section">
                            <h5>🔐 Login Form Details</h5>
                    `;

                    forms.login_form_details.forEach((form, index) => {
                        html += `
                            <div class="login-form-card">
                                <div class="login-form-header">
                                    <span class="form-badge">Form ${index + 1}</span>
                                    <span class="method-badge">${form.method}</span>
                                </div>
                                <div class="login-form-details">
                                    <div class="form-detail">
                                        <span class="form-detail-label">Action:</span>
                                        <code>${this.escapeHtml(form.action)}</code>
                                    </div>
                                    <div class="form-detail">
                                        <span class="form-detail-label">Fields:</span>
                                        <div class="field-tags">
                                            ${form.fields.map(field => `<span class="field-tag">${this.escapeHtml(field)}</span>`).join('')}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        `;
                    });

                    html += `</div>`;
                }

                html += `</div>`;
            }
        }

        this.analysisResults.innerHTML = html;

        setTimeout(() => {
            const counterElements = this.analysisResults.querySelectorAll('[data-counter]');
            counterElements.forEach((element, index) => {
                const target = parseInt(element.getAttribute('data-counter'), 10);
                setTimeout(() => {
                    this.animateCounter(element, target, 1000);
                }, index * 150);
            });
        }, 100);
    }

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    formatBytes(bytes) {
        if (bytes === 0) return '0 Bytes';

        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));

        return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
    }

    createPieChart(data, totalHeadings) {
        const width = 505;
        const height = 400;
        const radius = Math.min(height, height) / 2 - 40;

        const colors = {
            h1: '#FF6B6B',
            h2: '#4ECDC4',
            h3: '#45B7D1',
            h4: '#96CEB4',
            h5: '#FFEAA7',
            h6: '#DDA15E'
        };

        // Clear any existing chart
        d3.select('#pie-chart').selectAll('*').remove();

        const svg = d3.select('#pie-chart')
            .append('svg')
            .attr('viewBox', `0 0 ${width} ${height}`)
            .attr('class', 'pie-chart')
            .append('g')
            .attr('transform', `translate(${width / 3}, ${height / 2})`);

        // Create pie generator
        const pie = d3.pie()
            .value(d => d.count)
            .sort(null);

        // Create arc generator
        const arc = d3.arc()
            .innerRadius(0)
            .outerRadius(radius);

        // Create arc for hover effect
        const arcHover = d3.arc()
            .innerRadius(0)
            .outerRadius(radius + 10);

        // Create arc for labels
        const arcLabel = d3.arc()
            .innerRadius(radius * 0.6)
            .outerRadius(radius * 0.6);

        // Create pie slices
        const slices = svg.selectAll('.pie-slice')
            .data(pie(data))
            .enter()
            .append('g')
            .attr('class', 'pie-slice');

        // Add paths for slices
        slices.append('path')
            .attr('d', arc)
            .attr('fill', d => colors[d.data.level])
            .attr('stroke', 'white')
            .attr('stroke-width', 2)
            .style('cursor', 'pointer')
            .style('opacity', 0.9)
            .on('mouseenter', function(event, d) {
                d3.select(this)
                    .transition()
                    .duration(200)
                    .attr('d', arcHover)
                    .style('opacity', 1);

                // Show tooltip
                tooltip
                    .style('opacity', 1)
                    .html(`
                        <strong>${d.data.level.toUpperCase()}</strong><br/>
                        Count: ${d.data.count}<br/>
                        ${((d.data.count / totalHeadings) * 100).toFixed(1)}%
                    `)
                    .style('left', (event.pageX + 10) + 'px')
                    .style('top', (event.pageY - 10) + 'px');
            })
            .on('mousemove', function(event) {
                tooltip
                    .style('left', (event.pageX + 10) + 'px')
                    .style('top', (event.pageY - 10) + 'px');
            })
            .on('mouseleave', function() {
                d3.select(this)
                    .transition()
                    .duration(200)
                    .attr('d', arc)
                    .style('opacity', 0.9);

                tooltip.style('opacity', 0);
            });

        // Add labels
        slices.append('text')
            .attr('transform', d => `translate(${arcLabel.centroid(d)})`)
            .attr('text-anchor', 'middle')
            .attr('class', 'pie-label')
            .style('font-size', '14px')
            .style('font-weight', '700')
            .style('fill', '#1f2937')
            .style('pointer-events', 'none')
            .text(d => d.data.level.toUpperCase());

        // Add count text below label
        slices.append('text')
            .attr('transform', d => {
                const pos = arcLabel.centroid(d);
                return `translate(${pos[0]}, ${pos[1] + 16})`;
            })
            .attr('text-anchor', 'middle')
            .attr('class', 'pie-count')
            .style('font-size', '12px')
            .style('font-weight', '600')
            .style('fill', '#6b7280')
            .style('pointer-events', 'none')
            .text(d => d.data.count);

        // Create tooltip
        const tooltip = d3.select('body')
            .append('div')
            .attr('class', 'pie-tooltip')
            .style('opacity', 0);

        // Add legend
        const legend = svg.append('g')
            .attr('class', 'pie-legend')
            .attr('transform', `translate(${radius + 20}, ${-radius})`);

        const legendItems = legend.selectAll('.legend-item')
            .data(data)
            .enter()
            .append('g')
            .attr('class', 'legend-item')
            .attr('transform', (d, i) => `translate(0, ${i * 25})`);

        legendItems.append('rect')
            .attr('width', 18)
            .attr('height', 18)
            .attr('rx', 3)
            .attr('fill', d => colors[d.level])
            .style('opacity', 0.9);

        legendItems.append('text')
            .attr('x', 24)
            .attr('y', 9)
            .attr('dy', '0.35em')
            .style('font-size', '12px')
            .style('font-weight', '600')
            .style('fill', '#374151')
            .text(d => `${d.level.toUpperCase()}: ${d.count} (${((d.count / totalHeadings) * 100).toFixed(1)}%)`);
    }

    celebrate() {
        if (window.soundManager) {
            window.soundManager.playMarioVictory().catch(err => {
                console.log('Audio playback failed (user interaction required):', err);
            });
        }

        this.launchFireworks();

        this.showTrophy();
        this.showBanner('Analysis Complete! 🎉');
    }

    launchFireworks() {
        const duration = 5 * 1000;
        const animationEnd = Date.now() + duration;
        const defaults = { startVelocity: 30, spread: 360, ticks: 60, zIndex: 0 };

        const randomInRange = (min, max) => {
            return Math.random() * (max - min) + min;
        };

        const interval = setInterval(() => {
            const timeLeft = animationEnd - Date.now();

            if (timeLeft <= 0) {
                clearInterval(interval);

                return;
            }

            const particleCount = 50 * (timeLeft / duration);
            let colors = ['#FFD700', '#FFA500', '#FF69B4'];

            confetti({
                ...defaults,
                particleCount,
                origin: { x: randomInRange(0.1, 0.3), y: Math.random() - 0.2 }
            });
            confetti({
                ...defaults,
                particleCount,
                origin: { x: randomInRange(0.7, 0.9), y: Math.random() - 0.2 }
            });
            confetti({
                ...defaults,
                particleCount: 10,
                scalar: 1.1,
                shapes: ['star']
            });
            confetti({
                particleCount: 15,
                angle: 60,
                spread: 55,
                origin: { x: 0 },
                colors: colors
            });
            confetti({
                particleCount: 15,
                angle: 120,
                spread: 55,
                origin: { x: 1 },
                colors: colors
            });
        }, 250);
    }

    showTrophy() {
        const trophy = document.createElement('div');
        trophy.className = 'celebration-trophy';
        trophy.textContent = '🏆';
        document.body.appendChild(trophy);

        setTimeout(() => {
            trophy.style.transition = 'opacity 0.5s ease-out';
            trophy.style.opacity = '0';
            setTimeout(() => trophy.remove(), 500);
        }, 3000);
    }

    showBanner(message) {
        const banner = document.createElement('div');
        banner.className = 'celebration-banner';
        banner.textContent = message;
        document.body.appendChild(banner);

        setTimeout(() => {
            banner.style.transition = 'opacity 0.5s ease-out, transform 0.5s ease-out';
            banner.style.opacity = '0';
            banner.style.transform = 'translate(-50%, -50%) scale(0.8)';
            setTimeout(() => banner.remove(), 500);
        }, 2500);
    }

    animateCounter(element, target, duration = 1000) {
        const start = 0;
        const startTime = performance.now();

        const animate = (currentTime) => {
            const elapsed = currentTime - startTime;
            const progress = Math.min(elapsed / duration, 1);

            const easeOutQuart = 1 - Math.pow(1 - progress, 4);
            const current = Math.floor(start + (target - start) * easeOutQuart);

            element.textContent = current;
            element.classList.add('counting');

            if (progress < 1) {
                requestAnimationFrame(animate);
            } else {
                element.textContent = target;
                element.classList.remove('counting');
            }
        };

        requestAnimationFrame(animate);
    }

    initializeKonamiCode() {
        const konamiCode = ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a'];
        let konamiIndex = 0;

        document.addEventListener('keydown', (e) => {
            if (e.key === konamiCode[konamiIndex]) {
                konamiIndex++;

                if (konamiIndex === konamiCode.length) {
                    this.triggerMegaCelebration();
                    konamiIndex = 0;
                }
            } else {
                konamiIndex = 0;
            }
        });
    }

    triggerMegaCelebration() {
        if (window.soundManager) {
            window.soundManager.playPowerUp().catch(err => {
                console.log('Audio playback failed:', err);
            });

            setTimeout(() => {
                window.soundManager.playSuccess().catch(() => {});
                window.soundManager.playCoin().catch(() => {});
                window.soundManager.playFirework().catch(() => {});
            }, 300);
        }

        this.launchMegaFireworks();

        this.showTrophy();
        this.showBanner('🎮 KONAMI CODE! MEGA CELEBRATION! 🎮');

        setTimeout(() => {
            this.showBanner('⭐  YOU ARE AWESOME!  ⭐');
        }, 2500);
    }

    launchMegaFireworks() {
        const duration = 5000;
        const animationEnd = Date.now() + duration;
        const defaults = { startVelocity: 30, spread: 360, ticks: 60, zIndex: 0 };

        const randomInRange = (min, max) => {
            return Math.random() * (max - min) + min;
        };

        const interval = setInterval(() => {
            const timeLeft = animationEnd - Date.now();

            if (timeLeft <= 0) {
                clearInterval(interval);

                return;
            }

            const particleCount = 100 * (timeLeft / duration);

            confetti({
                ...defaults,
                particleCount,
                origin: { x: randomInRange(0.1, 0.3), y: Math.random() - 0.2 }
            });
            confetti({
                ...defaults,
                particleCount,
                origin: { x: randomInRange(0.4, 0.6), y: Math.random() - 0.2 }
            });
            confetti({
                ...defaults,
                particleCount,
                origin: { x: randomInRange(0.7, 0.9), y: Math.random() - 0.2 }
            });

            if (timeLeft > duration / 2) {
                let colors = ['#FFD700', '#FFA500', '#FF69B4'];
                confetti({
                    particleCount: 10,
                    angle: 60,
                    spread: 55,
                    origin: { x: 0 },
                    colors: colors
                });
                confetti({
                    particleCount: 10,
                    angle: 120,
                    spread: 55,
                    origin: { x: 1 },
                    colors: colors
                });
            }
        }, 250);
    }

    showResults(url, analysisId) {
        this.results.classList.remove('hidden');
        this.analysisInfo.innerHTML = `
            <p><strong>URL:</strong> ${url}</p>
            <p><strong>Analysis ID:</strong> <code>${analysisId}</code></p>
            <p><strong>Status:</strong> Analysis submitted and queued for processing</p>
        `;
    }

    showError(message) {
        this.error.classList.remove('hidden');
        this.errorMessage.textContent = message;
    }

    clearResults() {
        this.results.classList.add('hidden');
        this.error.classList.add('hidden');
        this.progressContainer.classList.add('hidden');
        this.analysisResults.classList.add('hidden');
        this.progressFill.style.width = '0%';
        this.progressText.textContent = '';
        this.progressLog.innerHTML = '';

        if (this.eventSource) {
            this.intentionallyClosed = true;
            this.eventSource.close();
            this.eventSource = null;
        }

        this.intentionallyClosed = false;
    }

    setLoading(loading) {
        this.analyzeButton.disabled = loading;

        if (loading) {
            this.buttonText.textContent = 'Analyzing...';
            this.spinner.classList.remove('hidden');
        } else {
            this.buttonText.textContent = 'Analyze';
            this.spinner.classList.add('hidden');
        }
    }
}

// Initialize the application when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    new WebAnalyzer();
});
