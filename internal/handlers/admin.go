package handlers

import (
"crypto/rand"
"encoding/hex"
"encoding/json"
"fmt"
"net/http"
"time"

"github.com/google/uuid"
"github.com/labstack/echo/v4"
"github.com/swecha/notifications/pkg/database"
"github.com/swecha/notifications/pkg/models"
"github.com/swecha/notifications/pkg/providers"
"github.com/swecha/notifications/pkg/queue"
)

type AdminHandler struct {
db    *database.DB
queue *queue.Queue
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *database.DB, q *queue.Queue) *AdminHandler {
return &AdminHandler{
db:    db,
queue: q,
}
}

// ===== API Key Management =====

// CreateAPIKeyRequest for creating new API keys
type CreateAPIKeyRequest struct {
AppID string `json:"app_id" validate:"required"`
}

// CreateAPIKeyResponse returns the generated key
type CreateAPIKeyResponse struct {
ID        string    `json:"id"`
AppID     string    `json:"app_id"`
Key       string    `json:"key"` // Only shown once
CreatedAt time.Time `json:"created_at"`
}

// CreateAPIKey generates a new API key for an app
func (h *AdminHandler) CreateAPIKey(c echo.Context) error {
var req CreateAPIKeyRequest
if err := c.Bind(&req); err != nil {
return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
}

// Generate API key
key := generateAPIKey()
keyHash := hashAPIKey(key)

// Save to database
apiKey := &models.APIKey{
ID:      uuid.New().String(),
AppID:   req.AppID,
KeyHash: keyHash,
}

if err := h.db.Save(apiKey).Error; err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create API key"})
}

return c.JSON(http.StatusCreated, CreateAPIKeyResponse{
ID:        apiKey.ID,
AppID:     apiKey.AppID,
Key:       key,
CreatedAt: apiKey.CreatedAt,
})
}

// GetAPIKeysResponse to list all API keys
type GetAPIKeysResponse struct {
ID         string     `json:"id"`
AppID      string     `json:"app_id"`
CreatedAt  time.Time  `json:"created_at"`
LastUsedAt *time.Time `json:"last_used_at"`
}

// GetAPIKeys lists all API keys
func (h *AdminHandler) GetAPIKeys(c echo.Context) error {
var keys []models.APIKey
if err := h.db.Find(&keys).Error; err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch API keys"})
}

response := make([]GetAPIKeysResponse, len(keys))
for i, key := range keys {
response[i] = GetAPIKeysResponse{
ID:         key.ID,
AppID:      key.AppID,
CreatedAt:  key.CreatedAt,
LastUsedAt: key.LastUsedAt,
}
}

return c.JSON(http.StatusOK, response)
}

// RevokeAPIKeyRequest to revoke a key
type RevokeAPIKeyRequest struct {
ID string `json:"id" validate:"required"`
}

// RevokeAPIKey revokes an API key
func (h *AdminHandler) RevokeAPIKey(c echo.Context) error {
var req RevokeAPIKeyRequest
if err := c.Bind(&req); err != nil {
return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
}

if err := h.db.Delete(&models.APIKey{}, "id = ?", req.ID).Error; err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to revoke API key"})
}

return c.JSON(http.StatusOK, map[string]string{"message": "API key revoked"})
}

// ===== Provider Configuration Management =====

// SaveProviderConfigRequest to save/update provider config
type SaveProviderConfigRequest struct {
ID       string                 `json:"id"`
AppID    string                 `json:"app_id"` // empty for global config
Provider string                 `json:"provider" validate:"required"`
Config   map[string]interface{} `json:"config" validate:"required"`
IsActive bool                   `json:"is_active"`
}

// SaveProviderConfigResponse returns saved config
type SaveProviderConfigResponse struct {
ID        string    `json:"id"`
AppID     string    `json:"app_id"`
Provider  string    `json:"provider"`
IsActive  bool      `json:"is_active"`
CreatedAt time.Time `json:"created_at"`
UpdatedAt time.Time `json:"updated_at"`
}

// SaveProviderConfig saves or updates provider configuration
func (h *AdminHandler) SaveProviderConfig(c echo.Context) error {
var req SaveProviderConfigRequest
if err := c.Bind(&req); err != nil {
return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
}

// Validate provider exists
if !isValidProvider(req.Provider) {
return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid provider"})
}

// Validate app_id if provided
var appID *string
if req.AppID != "" {
if _, err := h.db.GetApp(req.AppID); err != nil {
return c.JSON(http.StatusBadRequest, map[string]string{"error": "App not found"})
}
appID = &req.AppID
}

// Serialize config to JSON
configJSON, err := json.Marshal(req.Config)
if err != nil {
return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid config JSON"})
}

// Create or update config
id := req.ID
if id == "" {
id = uuid.New().String()
}

pc := &models.ProviderConfig{
ID:       id,
AppID:    appID,
Provider: req.Provider,
Config:   string(configJSON),
IsActive: req.IsActive,
}

if err := h.db.SaveProviderConfig(pc); err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save provider config"})
}

var respAppID string
if pc.AppID != nil {
respAppID = *pc.AppID
}

return c.JSON(http.StatusOK, SaveProviderConfigResponse{
ID:        pc.ID,
AppID:     respAppID,
Provider:  pc.Provider,
IsActive:  pc.IsActive,
CreatedAt: pc.CreatedAt,
UpdatedAt: pc.UpdatedAt,
})
}

// GetProviderConfigResponse returns provider config details
type GetProviderConfigResponse struct {
ID        string                 `json:"id"`
AppID     string                 `json:"app_id"`
Provider  string                 `json:"provider"`
Config    map[string]interface{} `json:"config"`
IsActive  bool                   `json:"is_active"`
CreatedAt time.Time              `json:"created_at"`
UpdatedAt time.Time              `json:"updated_at"`
}

// GetProviderConfigs lists all provider configurations
func (h *AdminHandler) GetProviderConfigs(c echo.Context) error {
var configs []models.ProviderConfig
if err := h.db.Find(&configs).Error; err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch configs"})
}

response := make([]GetProviderConfigResponse, len(configs))
for i, cfg := range configs {
var configMap map[string]interface{}
if err := json.Unmarshal([]byte(cfg.Config), &configMap); err != nil {
configMap = make(map[string]interface{})
}
var respAppID string
if cfg.AppID != nil {
respAppID = *cfg.AppID
}
response[i] = GetProviderConfigResponse{
ID:        cfg.ID,
AppID:     respAppID,
Provider:  cfg.Provider,
Config:    configMap,
IsActive:  cfg.IsActive,
CreatedAt: cfg.CreatedAt,
UpdatedAt: cfg.UpdatedAt,
}
}

return c.JSON(http.StatusOK, response)
}

// DeleteProviderConfigRequest to delete a config
type DeleteProviderConfigRequest struct {
ID string `json:"id" validate:"required"`
}

// DeleteProviderConfig deletes a provider configuration
func (h *AdminHandler) DeleteProviderConfig(c echo.Context) error {
var req DeleteProviderConfigRequest
if err := c.Bind(&req); err != nil {
return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
}

if err := h.db.Delete(&models.ProviderConfig{}, "id = ?", req.ID).Error; err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete config"})
}

return c.JSON(http.StatusOK, map[string]string{"message": "Provider config deleted"})
}

// ===== Notification Management =====

// GetNotificationsResponse paginated list of notifications
type GetNotificationsResponse struct {
ID             string     `json:"id"`
AppID          string     `json:"app_id"`
Provider       string     `json:"provider"`
Status         string     `json:"status"`
Recipient      string     `json:"recipient"`
Subject        string     `json:"subject"`
RetryCount     int        `json:"retry_count"`
ErrorMessage   string     `json:"error_message"`
CreatedAt      time.Time  `json:"created_at"`
DeliveredAt    *time.Time `json:"delivered_at"`
}

// GetNotifications lists notifications with filters
func (h *AdminHandler) GetNotifications(c echo.Context) error {
page := 1
pageSize := 20
status := c.QueryParam("status")
provider := c.QueryParam("provider")
appID := c.QueryParam("app_id")

if p := c.QueryParam("page"); p != "" {
_, _ = fmt.Sscanf(p, "%d", &page)
}
if ps := c.QueryParam("page_size"); ps != "" {
_, _ = fmt.Sscanf(ps, "%d", &pageSize)
}

query := h.db.DB

if status != "" {
query = query.Where("status = ?", status)
}
if provider != "" {
query = query.Where("provider = ?", provider)
}
if appID != "" {
query = query.Where("app_id = ?", appID)
}

var notifications []models.Notification
offset := (page - 1) * pageSize

if err := query.
Order("created_at DESC").
Offset(offset).
Limit(pageSize).
Find(&notifications).Error; err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch notifications"})
}

response := make([]GetNotificationsResponse, len(notifications))
for i, n := range notifications {
response[i] = GetNotificationsResponse{
ID:           n.ID,
AppID:        n.AppID,
Provider:     n.Provider,
Status:       n.Status,
Recipient:    n.Recipient,
Subject:      n.Subject,
RetryCount:   n.RetryCount,
ErrorMessage: n.ErrorMessage,
CreatedAt:    n.CreatedAt,
DeliveredAt:  n.DeliveredAt,
}
}

return c.JSON(http.StatusOK, response)
}

// ===== Dashboard Data =====

// DashboardStatsResponse contains dashboard statistics
type DashboardStatsResponse struct {
TotalNotifications    int64  `json:"total_notifications"`
PendingNotifications  int64  `json:"pending_notifications"`
SentNotifications     int64  `json:"sent_notifications"`
FailedNotifications   int64  `json:"failed_notifications"`
ActiveProviders       int64  `json:"active_providers"`
TotalAPIKeys          int64  `json:"total_api_keys"`
SuccessRate           string `json:"success_rate"`
AverageDeliveryTime   string `json:"average_delivery_time"`
}

// GetDashboardStats returns dashboard statistics
func (h *AdminHandler) GetDashboardStats(c echo.Context) error {
var total, pending, sent, failed int64
var activeProviders, totalKeys int64

h.db.Model(&models.Notification{}).Count(&total)
h.db.Model(&models.Notification{}).Where("status = ?", models.StatusPending).Count(&pending)
h.db.Model(&models.Notification{}).Where("status = ?", models.StatusSent).Count(&sent)
h.db.Model(&models.Notification{}).Where("status = ?", models.StatusFailed).Count(&failed)
h.db.Model(&models.ProviderConfig{}).Where("is_active = ?", true).Count(&activeProviders)
h.db.Model(&models.APIKey{}).Count(&totalKeys)

successRate := "0%"
if total > 0 {
successRate = fmt.Sprintf("%.2f%%", float64(sent)*100/float64(total))
}

avgDeliveryTime := "N/A"
var avgTime int64
if err := h.db.Model(&models.Notification{}).
Where("status = ?", models.StatusSent).
Where("delivered_at IS NOT NULL").
Select("EXTRACT(EPOCH FROM AVG(delivered_at - created_at))").
Row().
Scan(&avgTime); err == nil && avgTime > 0 {
avgDeliveryTime = fmt.Sprintf("%ds", avgTime)
}

return c.JSON(http.StatusOK, DashboardStatsResponse{
TotalNotifications:   total,
PendingNotifications: pending,
SentNotifications:    sent,
FailedNotifications:  failed,
ActiveProviders:      activeProviders,
TotalAPIKeys:         totalKeys,
SuccessRate:          successRate,
AverageDeliveryTime:  avgDeliveryTime,
})
}

// ===== Available Providers =====

// ProviderMetadataResponse describes available providers
type ProviderMetadataResponse struct {
Name           string                 `json:"name"`
Description    string                 `json:"description"`
RequiredFields []string               `json:"required_fields"`
OptionalFields []string               `json:"optional_fields"`
ExampleConfig  map[string]interface{} `json:"example_config"`
}

// GetAvailableProviders returns metadata about available providers
func (h *AdminHandler) GetAvailableProviders(c echo.Context) error {
registered := providers.GetRegistered()

// Map provider names to their metadata
metadata := map[string]ProviderMetadataResponse{
"smtp": {
Name:        "SMTP (Email)",
Description: "Send emails via SMTP server",
RequiredFields: []string{"host", "port", "username", "password", "from"},
OptionalFields: []string{"tls", "timeout"},
ExampleConfig: map[string]interface{}{
"host":     "smtp.gmail.com",
"port":     587,
"username": "your-email@gmail.com",
"password": "app-password",
"from":     "noreply@example.com",
"tls":      true,
},
},
"sms": {
Name:        "SMS",
Description: "Send SMS messages",
RequiredFields: []string{"provider", "api_key"},
OptionalFields: []string{"from_number"},
ExampleConfig: map[string]interface{}{
"provider":    "twilio",
"api_key":     "your-api-key",
"from_number": "+1234567890",
},
},
}

response := make([]ProviderMetadataResponse, 0)
for _, name := range registered {
if m, exists := metadata[name]; exists {
response = append(response, m)
}
}

return c.JSON(http.StatusOK, response)
}

// AdminDashboard serves the admin dashboard HTML
func (h *AdminHandler) AdminDashboard(c echo.Context) error {
return c.HTML(http.StatusOK, AdminHTML)
}

// ===== Utility Functions =====

// generateAPIKey creates a random API key
func generateAPIKey() string {
b := make([]byte, 32)
if _, err := rand.Read(b); err != nil {
panic(err)
}
return "sk_" + hex.EncodeToString(b)
}

// isValidProvider checks if provider exists
func isValidProvider(name string) bool {
registered := providers.GetRegistered()
for _, p := range registered {
if p == name {
return true
}
}
return false
}

// ===== API Key Request Management =====

// CreateAPIKeyRequestRequest for user to request API key
type CreateAPIKeyRequestRequest struct {
AppName     string `json:"app_name" validate:"required"`
AppID       string `json:"app_id" validate:"required"`
Email       string `json:"email" validate:"required,email"`
CompanyName string `json:"company_name"`
Purpose     string `json:"purpose" validate:"required"`
}

// CreateAPIKeyRequestResponse returns the request ID
type CreateAPIKeyRequestResponse struct {
ID        string    `json:"id"`
Status    string    `json:"status"`
CreatedAt time.Time `json:"created_at"`
Message   string    `json:"message"`
}

// RequestAPIKey creates a new API key request
func (h *AdminHandler) RequestAPIKey(c echo.Context) error {
var req CreateAPIKeyRequestRequest
if err := c.Bind(&req); err != nil {
return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
}

// Create request
apiKeyReq := &models.APIKeyRequest{
ID:          uuid.New().String(),
AppName:     req.AppName,
AppID:       req.AppID,
Email:       req.Email,
CompanyName: req.CompanyName,
Purpose:     req.Purpose,
Status:      "pending",
}

if err := h.db.SaveAPIKeyRequest(apiKeyReq); err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create request"})
}

return c.JSON(http.StatusCreated, CreateAPIKeyRequestResponse{
ID:        apiKeyReq.ID,
Status:    "pending",
CreatedAt: apiKeyReq.CreatedAt,
Message:   "API key request submitted. Admin will review and approve.",
})
}

// GetAPIKeyRequestResponse returns request details
type GetAPIKeyRequestResponse struct {
ID           string     `json:"id"`
AppName      string     `json:"app_name"`
AppID        string     `json:"app_id"`
Email        string     `json:"email"`
CompanyName  string     `json:"company_name"`
Purpose      string     `json:"purpose"`
Status       string     `json:"status"`
AdminComment string     `json:"admin_comment"`
CreatedAt    time.Time  `json:"created_at"`
UpdatedAt    time.Time  `json:"updated_at"`
ApprovedAt   *time.Time `json:"approved_at"`
ApprovedBy   string     `json:"approved_by"`
}

// GetAPIKeyRequests lists API key requests (admin view)
func (h *AdminHandler) GetAPIKeyRequests(c echo.Context) error {
status := c.QueryParam("status")
requests, err := h.db.GetAPIKeyRequests(status)
if err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch requests"})
}

response := make([]GetAPIKeyRequestResponse, len(requests))
for i, req := range requests {
response[i] = GetAPIKeyRequestResponse{
ID:           req.ID,
AppName:      req.AppName,
AppID:        req.AppID,
Email:        req.Email,
CompanyName:  req.CompanyName,
Purpose:      req.Purpose,
Status:       req.Status,
AdminComment: req.AdminComment,
CreatedAt:    req.CreatedAt,
UpdatedAt:    req.UpdatedAt,
ApprovedAt:   req.ApprovedAt,
ApprovedBy:   req.ApprovedBy,
}
}

return c.JSON(http.StatusOK, response)
}

// ApproveAPIKeyRequestRequest to approve/reject a request
type ApproveAPIKeyRequestRequest struct {
RequestID    string `json:"request_id" validate:"required"`
Action       string `json:"action" validate:"required,oneof=approve reject"`
AdminComment string `json:"admin_comment"`
}

// ApproveAPIKeyRequest approves or rejects an API key request
func (h *AdminHandler) ApproveAPIKeyRequest(c echo.Context) error {
var req ApproveAPIKeyRequestRequest
if err := c.Bind(&req); err != nil {
return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
}

// Get the request
apiKeyReq, err := h.db.GetAPIKeyRequest(req.RequestID)
if err != nil {
return c.JSON(http.StatusNotFound, map[string]string{"error": "Request not found"})
}

// Update status
status := req.Action
if status == "approve" {
// Create actual API key
key := generateAPIKey()
keyHash := hashAPIKey(key)

apiKey := &models.APIKey{
ID:      uuid.New().String(),
AppID:   apiKeyReq.AppID,
KeyHash: keyHash,
}

if err := h.db.Save(apiKey).Error; err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create API key"})
}

// Update request to approved
if err := h.db.UpdateAPIKeyRequest(req.RequestID, "approved", req.AdminComment, "admin"); err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update request"})
}

return c.JSON(http.StatusOK, map[string]interface{}{
"message": "API key request approved",
"api_key": map[string]interface{}{
"app_id": apiKeyReq.AppID,
"key":    key,
},
})
} else {
// Reject
if err := h.db.UpdateAPIKeyRequest(req.RequestID, "rejected", req.AdminComment, "admin"); err != nil {
return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to reject request"})
}

return c.JSON(http.StatusOK, map[string]string{
"message": "API key request rejected",
})
}
}

// GetRequestPage serves the public API key request form page
func (h *AdminHandler) GetRequestPage(c echo.Context) error {
return c.HTML(http.StatusOK, getRequestPageHTML())
}

// getRequestPageHTML returns the public API key request form
func getRequestPageHTML() string {
return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Request API Key - Notification Service</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .container { max-width: 500px; width: 100%; }
        .card {
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            padding: 40px;
        }
        h1 {
            font-size: 28px;
            margin-bottom: 10px;
            color: #333;
        }
        .subtitle {
            font-size: 14px;
            color: #999;
            margin-bottom: 30px;
        }
        .form-group { margin-bottom: 20px; }
        label {
            display: block;
            margin-bottom: 8px;
            font-weight: 500;
            font-size: 14px;
            color: #333;
        }
        input, textarea, select {
            width: 100%;
            padding: 10px 12px;
            border: 1px solid #ddd;
            border-radius: 6px;
            font-size: 14px;
            font-family: inherit;
        }
        input:focus, textarea:focus, select:focus {
            outline: none;
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
        }
        textarea { resize: vertical; min-height: 80px; }
        .btn {
            width: 100%;
            padding: 12px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s;
        }
        .btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }
        .btn:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
        }
        .alert {
            padding: 12px 15px;
            border-radius: 6px;
            margin-bottom: 20px;
            font-size: 14px;
        }
        .alert-success {
            background: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        .alert-error {
            background: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        .loading {
            display: inline-block;
            width: 20px;
            height: 20px;
            border: 3px solid rgba(255,255,255,0.3);
            border-radius: 50%;
            border-top-color: white;
            animation: spin 1s linear infinite;
        }
        @keyframes spin {
            to { transform: rotate(360deg); }
        }
        .success-box {
            text-align: center;
            padding: 20px;
        }
        .success-icon {
            font-size: 48px;
            margin-bottom: 15px;
        }
        .success-message {
            font-size: 16px;
            color: #155724;
            margin-bottom: 10px;
        }
        .success-details {
            font-size: 13px;
            color: #999;
            margin-top: 15px;
        }
        .info-box {
            background: #f9f9f9;
            padding: 12px;
            border-radius: 6px;
            font-size: 12px;
            color: #666;
            margin-bottom: 20px;
            line-height: 1.6;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="card">
            <h1>🔑 Request API Key</h1>
            <p class="subtitle">Fill out the form below to request an API key</p>
            
            <div id="form-section">
                <div class="info-box">
                    Your request will be reviewed by our admin team. You'll receive the API key once approved.
                </div>
                
                <form id="requestForm" onsubmit="submitRequest(event)">
                    <div class="form-group">
                        <label for="appName">Application Name *</label>
                        <input type="text" id="appName" placeholder="e.g., My Mobile App" required>
                    </div>

                    <div class="form-group">
                        <label for="appId">App ID *</label>
                        <input type="text" id="appId" placeholder="e.g., my-mobile-app" required>
                    </div>

                    <div class="form-group">
                        <label for="email">Email Address *</label>
                        <input type="email" id="email" placeholder="your@email.com" required>
                    </div>

                    <div class="form-group">
                        <label for="company">Company/Organization</label>
                        <input type="text" id="company" placeholder="Optional">
                    </div>

                    <div class="form-group">
                        <label for="purpose">Purpose/Use Case *</label>
                        <textarea id="purpose" placeholder="Describe how you'll use the API key..." required></textarea>
                    </div>

                    <button type="submit" class="btn" id="submitBtn">Request API Key</button>
                </form>
            </div>

            <div id="success-section" style="display: none;">
                <div class="success-box">
                    <div class="success-icon">✅</div>
                    <div class="success-message">Request Submitted!</div>
                    <div class="success-details" id="successMessage"></div>
                </div>
            </div>

            <div id="alert"></div>
        </div>
    </div>

    <script>
        const API_BASE = '';

        async function submitRequest(e) {
            e.preventDefault();
            
            const submitBtn = document.getElementById('submitBtn');
            submitBtn.disabled = true;
            submitBtn.innerHTML = '<div class="loading"></div>';

            try {
                const res = await fetch(API_BASE + '/api-key-request', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        app_name: document.getElementById('appName').value,
                        app_id: document.getElementById('appId').value,
                        email: document.getElementById('email').value,
                        company_name: document.getElementById('company').value,
                        purpose: document.getElementById('purpose').value
                    })
                });

                const data = await res.json();

                if (res.ok) {
                    document.getElementById('form-section').style.display = 'none';
                    document.getElementById('success-section').style.display = 'block';
                    document.getElementById('successMessage').innerHTML = 
                        '<strong>Request ID:</strong> ' + data.id + '<br>' +
                        '<strong>Status:</strong> ' + data.status + '<br>' +
                        'We\'ll review your request and send you an API key to ' + document.getElementById('email').value;
                } else {
                    showAlert(data.error || 'Failed to submit request', 'error');
                    submitBtn.disabled = false;
                    submitBtn.innerHTML = 'Request API Key';
                }
            } catch (err) {
                showAlert('Error: ' + err.message, 'error');
                submitBtn.disabled = false;
                submitBtn.innerHTML = 'Request API Key';
            }
        }

        function showAlert(message, type) {
            const alert = document.getElementById('alert');
            alert.className = 'alert alert-' + type;
            alert.textContent = message;
            alert.style.display = 'block';
        }
    </script>
</body>
</html>`
}
