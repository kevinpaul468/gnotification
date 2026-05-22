package handlers

const AdminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Notification Admin Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background: #f5f5f5; color: #333; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 20px 30px; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        h1 { font-size: 28px; margin-bottom: 8px; }
        .nav-tabs { display: flex; gap: 0; margin-top: 20px; border-bottom: 2px solid rgba(255,255,255,0.2); }
        .nav-tab { background: none; border: none; color: white; padding: 12px 20px; cursor: pointer; font-size: 14px; font-weight: 500; border-bottom: 3px solid transparent; margin-bottom: -2px; transition: all 0.3s; }
        .nav-tab:hover { background: rgba(255,255,255,0.1); }
        .nav-tab.active { border-bottom-color: white; }
        .content { background: white; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); margin-top: 20px; padding: 30px; }
        .tab-content { display: none; }
        .tab-content.active { display: block; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .stat-card { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .stat-card.secondary { background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); }
        .stat-label { font-size: 12px; opacity: 0.9; margin-bottom: 8px; text-transform: uppercase; letter-spacing: 1px; }
        .stat-value { font-size: 32px; font-weight: bold; }
        .form-section { margin-bottom: 30px; }
        .form-section h3 { font-size: 18px; margin-bottom: 15px; color: #333; border-bottom: 2px solid #667eea; padding-bottom: 10px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: 500; font-size: 14px; }
        .form-group input, .form-group select, .form-group textarea { width: 100%; padding: 8px 12px; border: 1px solid #ddd; border-radius: 4px; font-size: 14px; font-family: inherit; }
        .form-group input:focus, .form-group select:focus, .form-group textarea:focus { outline: none; border-color: #667eea; box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1); }
        .btn { padding: 10px 20px; border: none; border-radius: 4px; font-size: 14px; font-weight: 500; cursor: pointer; transition: all 0.3s; }
        .btn-primary { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; }
        .btn-primary:hover { transform: translateY(-2px); box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4); }
        .btn-secondary { background: #f0f0f0; color: #333; }
        .btn-secondary:hover { background: #e0e0e0; }
        .btn-danger { background: #f5576c; color: white; }
        .btn-danger:hover { background: #e63a52; }
        .btn-success { background: #4CAF50; color: white; }
        .btn-success:hover { background: #45a049; }
        .table { width: 100%; border-collapse: collapse; margin-top: 15px; }
        .table th { background: #f5f5f5; padding: 12px; text-align: left; font-weight: 600; font-size: 13px; border-bottom: 2px solid #ddd; }
        .table td { padding: 12px; border-bottom: 1px solid #eee; font-size: 13px; }
        .table tr:hover { background: #fafafa; }
        .badge { display: inline-block; padding: 4px 12px; border-radius: 12px; font-size: 12px; font-weight: 500; }
        .badge-success { background: #d4edda; color: #155724; }
        .badge-pending { background: #fff3cd; color: #856404; }
        .badge-failed { background: #f8d7da; color: #721c24; }
        .badge-active { background: #d4edda; color: #155724; }
        .badge-inactive { background: #e2e3e5; color: #383d41; }
        .badge-approved { background: #d4edda; color: #155724; }
        .badge-rejected { background: #f8d7da; color: #721c24; }
        .no-data { text-align: center; padding: 40px; color: #999; }
        .grid-form { display: grid; grid-template-columns: 1fr 1fr; gap: 15px; }
        .action-buttons { display: flex; gap: 5px; }
        .action-buttons button { padding: 5px 10px; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>📨 Notification Admin Dashboard</h1>
        <div class="nav-tabs">
            <button class="nav-tab active" onclick="switchTab(event, 'dashboard')">Dashboard</button>
            <button class="nav-tab" onclick="switchTab(event, 'api-requests')">API Requests</button>
            <button class="nav-tab" onclick="switchTab(event, 'api-keys')">API Keys</button>
            <button class="nav-tab" onclick="switchTab(event, 'providers')">Providers</button>
            <button class="nav-tab" onclick="switchTab(event, 'notifications')">Notifications</button>
        </div>
    </div>

    <div class="container">
        <div id="dashboard" class="tab-content active">
            <div class="content">
                <h2>Dashboard Overview</h2>
                <div class="stats-grid" id="statsGrid">
                    <div class="no-data">Loading statistics...</div>
                </div>
            </div>
        </div>

        <div id="api-requests" class="tab-content">
            <div class="content">
                <h2>API Key Requests</h2>
                <div class="form-section">
                    <h3>Filter Requests</h3>
                    <div class="grid-form">
                        <div class="form-group">
                            <label for="filterRequestStatus">Status</label>
                            <select id="filterRequestStatus" onchange="loadAPIKeyRequests()">
                                <option value="">All</option>
                                <option value="pending">Pending</option>
                                <option value="approved">Approved</option>
                                <option value="rejected">Rejected</option>
                            </select>
                        </div>
                    </div>
                </div>
                <table class="table">
                    <thead>
                        <tr>
                            <th>App Name</th>
                            <th>App ID</th>
                            <th>Email</th>
                            <th>Company</th>
                            <th>Status</th>
                            <th>Created</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody id="apiRequestsBody">
                        <tr><td colspan="7" class="no-data">Loading...</td></tr>
                    </tbody>
                </table>
            </div>
        </div>

        <div id="api-keys" class="tab-content">
            <div class="content">
                <h2>API Key Management</h2>
                <div class="form-section">
                    <h3>Create New API Key</h3>
                    <div class="form-group">
                        <label for="appId">App ID</label>
                        <input type="text" id="appId" placeholder="e.g., my-app">
                    </div>
                    <button class="btn btn-primary" onclick="createAPIKey()">Generate API Key</button>
                </div>
                <div class="form-section">
                    <h3>Active API Keys</h3>
                    <table class="table">
                        <thead>
                            <tr><th>App ID</th><th>Created</th><th>Last Used</th><th>Actions</th></tr>
                        </thead>
                        <tbody id="apiKeysBody">
                            <tr><td colspan="4" class="no-data">Loading...</td></tr>
                        </tbody>
                    </table>
                </div>
            </div>
        </div>

        <div id="providers" class="tab-content">
            <div class="content">
                <h2>Provider Management</h2>
                <div class="form-section">
                    <h3>Active Configurations</h3>
                    <table class="table">
                        <thead>
                            <tr><th>Provider</th><th>Status</th><th>Created</th><th>Actions</th></tr>
                        </thead>
                        <tbody id="providersBody">
                            <tr><td colspan="4" class="no-data">Loading...</td></tr>
                        </tbody>
                    </table>
                </div>
            </div>
        </div>

        <div id="notifications" class="tab-content">
            <div class="content">
                <h2>Notifications</h2>
                <table class="table">
                    <thead>
                        <tr><th>ID</th><th>App</th><th>Provider</th><th>Status</th><th>Created</th></tr>
                    </thead>
                    <tbody id="notificationsBody">
                        <tr><td colspan="5" class="no-data">Loading...</td></tr>
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <script>
        const API_BASE = '';

        function switchTab(e, tabName) {
            e.preventDefault();
            document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
            document.querySelectorAll('.nav-tab').forEach(el => el.classList.remove('active'));
            
            const tab = document.getElementById(tabName);
            if (tab) tab.classList.add('active');
            e.target.classList.add('active');

            if (tabName === 'dashboard') loadStats();
            if (tabName === 'api-requests') loadAPIKeyRequests();
            if (tabName === 'api-keys') loadAPIKeys();
            if (tabName === 'providers') loadProviders();
            if (tabName === 'notifications') loadNotifications();
        }

        async function loadStats() {
            try {
                const res = await fetch(API_BASE + '/admin/stats');
                const stats = await res.json();
                let html = '<div class="stat-card"><div class="stat-label">Total</div><div class="stat-value">' + stats.total_notifications + '</div></div>';
                html += '<div class="stat-card secondary"><div class="stat-label">Pending</div><div class="stat-value">' + stats.pending_notifications + '</div></div>';
                html += '<div class="stat-card"><div class="stat-label">Sent</div><div class="stat-value">' + stats.sent_notifications + '</div></div>';
                html += '<div class="stat-card secondary"><div class="stat-label">Failed</div><div class="stat-value">' + stats.failed_notifications + '</div></div>';
                document.getElementById('statsGrid').innerHTML = html;
            } catch (err) {
                document.getElementById('statsGrid').innerHTML = '<div class="no-data">Error loading stats</div>';
            }
        }

        async function loadAPIKeyRequests() {
            try {
                const status = document.getElementById('filterRequestStatus').value;
                let url = API_BASE + '/admin/api-key-requests';
                if (status) url += '?status=' + status;
                
                const res = await fetch(url);
                const requests = await res.json() || [];

                let html = '';
                if (requests.length === 0) {
                    html = '<tr><td colspan="7" class="no-data">No requests</td></tr>';
                } else {
                    requests.forEach(req => {
                        const badge = 'badge-' + req.status;
                        html += '<tr><td>' + req.app_name + '</td><td>' + req.app_id + '</td><td>' + req.email + '</td><td>' + req.company_name + '</td>';
                        html += '<td><span class="badge ' + badge + '">' + req.status + '</span></td>';
                        html += '<td>' + new Date(req.created_at).toLocaleString() + '</td><td>';
                        if (req.status === 'pending') {
                            html += '<button class="btn btn-success" onclick="approveRequest(\'' + req.id + '\')">Approve</button> ';
                            html += '<button class="btn btn-danger" onclick="rejectRequest(\'' + req.id + '\')">Reject</button>';
                        }
                        html += '</td></tr>';
                    });
                }
                document.getElementById('apiRequestsBody').innerHTML = html;
            } catch (err) {
                document.getElementById('apiRequestsBody').innerHTML = '<tr><td colspan="7" class="no-data">Error loading requests</td></tr>';
            }
        }

        async function approveRequest(requestId) {
            const comment = prompt('Add admin comment (optional):');
            try {
                const res = await fetch(API_BASE + '/admin/api-key-requests/approve', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        request_id: requestId,
                        action: 'approve',
                        admin_comment: comment || ''
                    })
                });
                if (res.ok) {
                    const data = await res.json();
                    alert('APPROVED!\\nAPI Key: ' + data.api_key.key + '\\n\\nSave this key securely!');
                    loadAPIKeyRequests();
                }
            } catch (err) {
                alert('Error: ' + err.message);
            }
        }

        async function rejectRequest(requestId) {
            const comment = prompt('Reason for rejection:');
            if (!comment) return;
            try {
                const res = await fetch(API_BASE + '/admin/api-key-requests/approve', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        request_id: requestId,
                        action: 'reject',
                        admin_comment: comment
                    })
                });
                if (res.ok) {
                    alert('Request rejected!');
                    loadAPIKeyRequests();
                }
            } catch (err) {
                alert('Error: ' + err.message);
            }
        }

        async function loadAPIKeys() {
            try {
                const res = await fetch(API_BASE + '/admin/api-keys');
                const keys = await res.json() || [];
                let html = '';
                if (keys.length === 0) html = '<tr><td colspan="4" class="no-data">No keys</td></tr>';
                else {
                    keys.forEach(key => {
                        html += '<tr><td>' + key.app_id + '</td><td>' + new Date(key.created_at).toLocaleDateString() + '</td>';
                        html += '<td>' + (key.last_used_at ? new Date(key.last_used_at).toLocaleDateString() : 'Never') + '</td>';
                        html += '<td><button class="btn btn-danger" onclick="revokeKey(\'' + key.id + '\')">Revoke</button></td></tr>';
                    });
                }
                document.getElementById('apiKeysBody').innerHTML = html;
            } catch (err) {}
        }

        async function createAPIKey() {
            const appId = document.getElementById('appId').value;
            if (!appId) return alert('Enter App ID');
            try {
                const res = await fetch(API_BASE + '/admin/api-keys', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ app_id: appId })
                });
                if (res.ok) {
                    const key = await res.json();
                    alert('API Key Created:\\n' + key.key + '\\n\\nSave it securely!');
                    document.getElementById('appId').value = '';
                    loadAPIKeys();
                }
            } catch (err) {}
        }

        async function revokeKey(keyId) {
            if (confirm('Revoke this key?')) {
                try {
                    await fetch(API_BASE + '/admin/api-keys', {
                        method: 'DELETE',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ id: keyId })
                    });
                    loadAPIKeys();
                } catch (err) {}
            }
        }

        async function loadProviders() {
            try {
                const res = await fetch(API_BASE + '/admin/provider-configs');
                const configs = await res.json() || [];
                let html = '';
                if (configs.length === 0) html = '<tr><td colspan="4" class="no-data">No providers</td></tr>';
                else {
                    configs.forEach(cfg => {
                        html += '<tr><td>' + cfg.provider + '</td>';
                        html += '<td><span class="badge ' + (cfg.is_active ? 'badge-active' : 'badge-inactive') + '">' + (cfg.is_active ? 'Active' : 'Inactive') + '</span></td>';
                        html += '<td>' + new Date(cfg.created_at).toLocaleDateString() + '</td>';
                        html += '<td><button class="btn btn-danger" onclick="deleteProvider(\'' + cfg.id + '\')">Delete</button></td></tr>';
                    });
                }
                document.getElementById('providersBody').innerHTML = html;
            } catch (err) {}
        }

        async function deleteProvider(id) {
            if (confirm('Delete this provider config?')) {
                try {
                    await fetch(API_BASE + '/admin/provider-configs', {
                        method: 'DELETE',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ id })
                    });
                    loadProviders();
                } catch (err) {}
            }
        }

        async function loadNotifications() {
            try {
                const res = await fetch(API_BASE + '/admin/notifications');
                const notifs = await res.json() || [];
                let html = '';
                if (notifs.length === 0) html = '<tr><td colspan="5" class="no-data">No notifications</td></tr>';
                else {
                    notifs.forEach(n => {
                        html += '<tr><td>' + n.id.substring(0, 8) + '...</td><td>' + n.app_id + '</td><td>' + n.provider + '</td>';
                        html += '<td><span class="badge badge-' + n.status + '">' + n.status + '</span></td>';
                        html += '<td>' + new Date(n.created_at).toLocaleString() + '</td></tr>';
                    });
                }
                document.getElementById('notificationsBody').innerHTML = html;
            } catch (err) {}
        }

        window.addEventListener('load', loadStats);
    </script>
</body>
</html>`
