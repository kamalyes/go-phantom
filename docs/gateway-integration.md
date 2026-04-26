# 网关集成

## 🎯 概述

go-phantom 提供了 HTTP 中间件和 gRPC 拦截器，可以无缝集成到现有的网关架构中，自动从请求中提取路由上下文，实现声明式的数据源切换

## 🌐 HTTP 中间件

### 基本使用

```go
import (
    "net/http"

    "github.com/kamalyes/go-phantom"
)

func main() {
    p := phantom.NewPhantom()
    // ... 注册分组和数据源 ...

    // 创建网关中间件
    gm := phantom.NewGatewayMiddleware(p, "X-Phantom-")

    // 使用中间件
    handler := gm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 在这里，路由上下文已经自动注入到 r.Context() 中
        source, _ := p.Resolve(r.Context(), "db_group")
        fmt.Fprintf(w, "当前数据源: %s", source.Name)
    }))

    http.Handle("/api", handler)
    http.ListenAndServe(":8080", nil)
}
```

### 请求头约定

默认前缀为 `X-Phantom-`，可以通过构造函数自定义：

| 请求头 | 说明 | 示例 |
|--------|------|------|
| X-Phantom-DS | 指定数据源名称 | `slave_db` |
| X-Phantom-Shadow | 影子流量标记 | `true` 或 `1` |
| X-Phantom-Tenant | 租户ID | `tenant_1` |
| X-Phantom-ReadOnly | 只读标记 | `true` 或 `1` |
| X-Phantom-Hint | 路由提示 | `hint_ds` |

### 自定义前缀

```go
gm := phantom.NewGatewayMiddleware(p, "X-Custom-")
// 请求头变为: X-Custom-DS, X-Custom-Shadow, X-Custom-Tenant 等
```

### 完整示例

```go
func main() {
    p := phantom.NewPhantom()

    dbGroup := phantom.NewGroup("db_group", phantom.StorageDatabase)
    dbGroup.AddSource(&phantom.DataSource{Name: "primary_db", StorageType: phantom.StorageDatabase})
    dbGroup.AddSource(&phantom.DataSource{Name: "slave_db", StorageType: phantom.StorageDatabase, ReadOnly: true})
    dbGroup.AddSource(&phantom.DataSource{Name: "shadow_db", StorageType: phantom.StorageDatabase, Shadow: true})
    p.RegisterGroup(dbGroup)
    p.SetDefaultGroup(phantom.StorageDatabase, "db_group")
    p.SetGroupStrategy("db_group", "readwrite")
    p.Initialize(context.Background())

    gm := phantom.NewGatewayMiddleware(p, "X-Phantom-")

    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        source, _ := p.Resolve(r.Context(), "db_group")
        fmt.Fprintf(w, "数据源: %s\n", source.Name)
    })

    handler := gm.Middleware(mux)
    http.ListenAndServe(":8080", handler)
}
```

### 客户端请求示例

```bash
# 默认路由到主库
curl http://localhost:8080/api/users

# 切换到从库（只读）
curl -H "X-Phantom-DS: slave_db" http://localhost:8080/api/users

# 影子流量
curl -H "X-Phantom-Shadow: true" http://localhost:8080/api/users

# 多租户
curl -H "X-Phantom-Tenant: tenant_1" http://localhost:8080/api/users

# 只读请求
curl -H "X-Phantom-ReadOnly: true" http://localhost:8080/api/users
```

## 🔌 gRPC 拦截器

### 基本使用

```go
import (
    "google.golang.org/grpc"

    "github.com/kamalyes/go-phantom"
)

func main() {
    p := phantom.NewPhantom()
    // ... 注册分组和数据源 ...

    // 创建 gRPC 拦截器
    gi := phantom.NewGRPCInterceptor(p, "phantom-")

    // 注册一元拦截器
    server := grpc.NewServer(
        grpc.UnaryInterceptor(gi.UnaryInterceptor()),
    )

    // 注册流拦截器
    server := grpc.NewServer(
        grpc.StreamInterceptor(gi.StreamInterceptor()),
    )
}
```

### Metadata 约定

默认前缀为 `phantom-`，可以通过构造函数自定义：

| Metadata 键 | 说明 | 示例 |
|-------------|------|------|
| phantom-ds | 指定数据源名称 | `slave_db` |
| phantom-shadow | 影子流量标记 | `true` |
| phantom-tenant | 租户ID | `tenant_1` |
| phantom-readonly | 只读标记 | `true` |
| phantom-hint | 路由提示 | `hint_ds` |

### 自定义前缀

```go
gi := phantom.NewGRPCInterceptor(p, "x-custom-")
// Metadata 键变为: x-custom-ds, x-custom-shadow 等
```

### 手动提取路由上下文

```go
// 在 gRPC 服务方法中手动提取
func (s *MyService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    // 拦截器已自动提取路由上下文
    source, _ := p.Resolve(ctx, "db_group")

    // 或手动提取
    routeCtx := gi.ExtractRouteContext(ctx)

    // ...
}
```

### 客户端请求示例

```go
// gRPC 客户端设置 metadata
md := metadata.Pairs(
    "phantom-ds", "slave_db",
    "phantom-tenant", "tenant_1",
)
ctx := metadata.NewOutgoingContext(context.Background(), md)

response, err := client.GetUser(ctx, &pb.GetUserRequest{Id: "1"})
```

## 🔗 与 Gin 框架集成

```go
import "github.com/gin-gonic/gin"

func main() {
    p := phantom.NewPhantom()
    // ... 注册分组和数据源 ...

    gm := phantom.NewGatewayMiddleware(p, "X-Phantom-")

    r := gin.Default()

    // 使用 Gin 中间件包装
    r.Use(func(c *gin.Context) {
        gm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            c.Request = r
            c.Next()
        })).ServeHTTP(c.Writer, c.Request)
    })

    r.GET("/api/users", func(c *gin.Context) {
        source, _ := p.Resolve(c.Request.Context(), "db_group")
        c.JSON(200, gin.H{"datasource": source.Name})
    })

    r.Run(":8080")
}
```

## 🔗 与 Echo 框架集成

```go
import "github.com/labstack/echo/v4"

func main() {
    p := phantom.NewPhantom()
    // ... 注册分组和数据源 ...

    gm := phantom.NewGatewayMiddleware(p, "X-Phantom-")

    e := echo.New()

    e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            req := c.Request()
            ctx := gm.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                c.SetRequest(r)
            }))
            ctx.ServeHTTP(c.Response(), req)
            return next(c)
        }
    })

    e.GET("/api/users", func(c echo.Context) error {
        source, _ := p.Resolve(c.Request().Context(), "db_group")
        return c.JSON(200, map[string]string{"datasource": source.Name})
    })

    e.Start(":8080")
}
```
