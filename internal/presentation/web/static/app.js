// Check if we're on the result page with a pending link check job
document.addEventListener('DOMContentLoaded', function() {
    const linkCheckStatus = document.getElementById('linkCheckStatus');

    if (linkCheckStatus) {
        const jobId = linkCheckStatus.dataset.jobId;
        if (jobId) {
            pollLinkCheckStatus(jobId);
        }
    }
});

// Poll for link check job completion
function pollLinkCheckStatus(jobId) {
    const statusText = document.getElementById('statusText');
    const resultsDiv = document.getElementById('checkResults');

    // Poll every 2 seconds, up to 30 times (1 minute)
    let attempts = 0;
    const maxAttempts = 30;

    const interval = setInterval(async () => {
        attempts++;

        if (attempts > maxAttempts) {
            clearInterval(interval);
            statusText.textContent = 'Timeout - job took too long';
            return;
        }

        try {
            const response = await fetch(`/api/jobs/${jobId}`);

            if (!response.ok) {
                if (response.status === 404) {
                    // Job not found yet, keep polling
                    return;
                }
                throw new Error(`HTTP ${response.status}`);
            }

            const job = await response.json();
            statusText.textContent = job.status;

            if (job.status === 'completed' || job.status === 'failed') {
                clearInterval(interval);

                if (job.result) {
                    displayLinkCheckResults(job.result, resultsDiv);
                }
            }
        } catch (error) {
            console.error('Failed to poll job status:', error);
        }
    }, 2000);
}

// Display link check results
function displayLinkCheckResults(result, container) {
    const html = `
        <div class="link-check-results">
            <h4>Link Check Complete</h4>
            <div class="info-grid">
                <div class="info-item">
                    <span class="label">Checked:</span>
                    <span class="value">${result.checked}</span>
                </div>
                <div class="info-item">
                    <span class="label">Accessible:</span>
                    <span class="value badge badge-success">${result.accessible}</span>
                </div>
                <div class="info-item">
                    <span class="label">Inaccessible:</span>
                    <span class="value badge badge-error">${result.inaccessible ? result.inaccessible.length : 0}</span>
                </div>
                <div class="info-item">
                    <span class="label">Duration:</span>
                    <span class="value">${result.duration}</span>
                </div>
            </div>
            ${result.inaccessible && result.inaccessible.length > 0 ? `
                <details class="broken-links">
                    <summary>Broken Links (${result.inaccessible.length})</summary>
                    <ul>
                        ${result.inaccessible.map(link => `
                            <li>
                                <code>${escapeHtml(link.url)}</code>
                                <span class="badge badge-error">${escapeHtml(link.reason)}</span>
                            </li>
                        `).join('')}
                    </ul>
                </details>
            ` : ''}
        </div>
    `;

    container.innerHTML = html;
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
