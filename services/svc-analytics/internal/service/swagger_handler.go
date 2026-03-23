// swagger_handler.go 提供 Swagger UI 静态页面和 OpenAPI YAML 文件服务。
// 通过 CDN 加载 swagger-ui-dist，无需本地静态文件依赖。
package service

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// SwaggerHandler 提供 API 文档站点的 HTTP 处理器。
// 支持两个端点：
//   - GET /api/docs       → 返回 Swagger UI HTML 页面
//   - GET /api/docs/{svc} → 返回对应服务的 OpenAPI YAML 文件
type SwaggerHandler struct {
	docsDir string     // OpenAPI YAML 文件所在目录（docs/api/）
	logger  *zap.Logger
}

// NewSwaggerHandler 创建 Swagger 文档处理器实例。
// docsDir 为 OpenAPI YAML 文件存放目录的绝对路径或相对路径。
func NewSwaggerHandler(docsDir string, logger *zap.Logger) *SwaggerHandler {
	return &SwaggerHandler{
		docsDir: docsDir,
		logger:  logger,
	}
}

// RegisterRoutes 将 Swagger 文档相关路由挂载到 chi 路由器。
func (s *SwaggerHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/docs", s.serveSwaggerUI)
	r.Get("/api/docs/{svc}", s.serveOpenAPISpec)
}

// swaggerUIHTML 是内嵌的 Swagger UI HTML 模板。
// 通过 CDN 加载 swagger-ui-dist 资源，默认展示 svc-log 服务的 API 文档。
// 页面包含服务选择下拉框，支持在不同服务的 API 文档之间切换。
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>OpsNexus API 文档</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; padding: 0; }
    .topbar-wrapper { display: flex; align-items: center; }
    .service-selector {
      margin: 10px 20px;
      padding: 8px 16px;
      font-size: 14px;
      border: 1px solid #ccc;
      border-radius: 4px;
    }
    .header-bar {
      background: #1b1b1b;
      padding: 10px 20px;
      display: flex;
      align-items: center;
      gap: 16px;
    }
    .header-bar h1 {
      color: #fff;
      margin: 0;
      font-size: 18px;
    }
    .header-bar select {
      padding: 6px 12px;
      font-size: 14px;
      border-radius: 4px;
      border: none;
    }
  </style>
</head>
<body>
  <div class="header-bar">
    <h1>OpsNexus API 文档</h1>
    <select id="service-select" class="service-selector" onchange="loadSpec()">
      <option value="svc-log">日志服务 (svc-log)</option>
      <option value="svc-alert">告警服务 (svc-alert)</option>
      <option value="svc-incident">事件服务 (svc-incident)</option>
      <option value="svc-analytics">分析服务 (svc-analytics)</option>
      <option value="svc-cmdb">CMDB 服务 (svc-cmdb)</option>
      <option value="svc-notify">通知服务 (svc-notify)</option>
      <option value="svc-ai">AI 服务 (svc-ai)</option>
    </select>
  </div>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    // 初始化 Swagger UI 实例
    let ui;
    function loadSpec() {
      const svc = document.getElementById('service-select').value;
      const specUrl = '/api/docs/' + svc;
      if (ui) {
        ui.specActions.updateUrl(specUrl);
        ui.specActions.download();
      } else {
        ui = SwaggerUIBundle({
          url: specUrl,
          dom_id: '#swagger-ui',
          deepLinking: true,
          presets: [
            SwaggerUIBundle.presets.apis,
            SwaggerUIBundle.SwaggerUIStandalonePreset
          ],
          layout: 'BaseLayout'
        });
      }
    }
    // 页面加载时初始化
    loadSpec();
  </script>
</body>
</html>`

// serveSwaggerUI 返回 Swagger UI HTML 页面。
func (s *SwaggerHandler) serveSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(swaggerUIHTML))
}

// serveOpenAPISpec 返回指定服务的 OpenAPI YAML 文件内容。
// URL 参数 {svc} 对应 docs/api/ 目录下的 YAML 文件名（不含扩展名）。
// 例如：GET /api/docs/svc-log → 读取 docs/api/svc-log.yaml
func (s *SwaggerHandler) serveOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	svc := chi.URLParam(r, "svc")

	// 安全校验：防止路径穿越攻击，只允许字母、数字和连字符
	for _, c := range svc {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
			http.Error(w, "invalid service name", http.StatusBadRequest)
			return
		}
	}

	// 构建 YAML 文件路径
	filename := svc + ".yaml"
	filePath := filepath.Join(s.docsDir, filename)

	// 规范化路径并验证其在 docsDir 范围内（防止路径穿越）
	absDocsDir, err := filepath.Abs(s.docsDir)
	if err != nil {
		s.logger.Error("无法获取文档目录绝对路径", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		s.logger.Error("无法获取文件绝对路径", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !strings.HasPrefix(absFilePath, absDocsDir) {
		http.Error(w, "invalid service name", http.StatusBadRequest)
		return
	}

	// 读取 YAML 文件
	data, err := os.ReadFile(absFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Warn("OpenAPI 文件不存在", zap.String("service", svc), zap.String("path", absFilePath))
			http.Error(w, "API spec not found for service: "+svc, http.StatusNotFound)
			return
		}
		s.logger.Error("读取 OpenAPI 文件失败", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
