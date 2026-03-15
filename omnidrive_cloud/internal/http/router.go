package http

import (
	stdhttp "net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/http/handlers"
	authmiddleware "omnidrive_cloud/internal/http/middleware"
)

func NewRouter(app *appstate.App) stdhttp.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	healthHandler := handlers.NewHealthHandler(app)
	authHandler := handlers.NewAuthHandler(app)
	deviceHandler := handlers.NewDeviceHandler(app)
	accountHandler := handlers.NewAccountHandler(app)
	skillHandler := handlers.NewSkillHandler(app)
	taskHandler := handlers.NewTaskHandler(app)
	agentHandler := handlers.NewAgentHandler(app)
	fileHandler := handlers.NewFileHandler(app)

	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)
	r.Get("/api/v1/files/*", fileHandler.Get)

	r.Route("/api/v1", func(api chi.Router) {
		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/register", authHandler.Register)
			auth.Post("/login", authHandler.Login)
			auth.With(authmiddleware.RequireUser(app)).Get("/me", authHandler.Me)
		})

		api.Group(func(private chi.Router) {
			private.Use(authmiddleware.RequireUser(app))

			private.Route("/devices", func(devices chi.Router) {
				devices.Get("/", deviceHandler.List)
				devices.Post("/claim", deviceHandler.Claim)
				devices.Patch("/{deviceId}", deviceHandler.Update)
			})

			private.Route("/accounts", func(accounts chi.Router) {
				accounts.Get("/", accountHandler.List)
				accounts.Post("/remote-login", accountHandler.CreateRemoteLogin)
				accounts.Get("/login-sessions/{sessionId}", accountHandler.GetLoginSession)
				accounts.Post("/login-sessions/{sessionId}/actions", accountHandler.CreateLoginAction)
			})

			private.Route("/skills", func(skills chi.Router) {
				skills.Get("/", skillHandler.List)
				skills.Post("/", skillHandler.Create)
				skills.Patch("/{skillId}", skillHandler.Update)
				skills.Get("/{skillId}/assets", skillHandler.ListAssets)
				skills.Post("/{skillId}/assets", skillHandler.CreateAsset)
				skills.Post("/{skillId}/upload", skillHandler.UploadAsset)
			})

			private.Route("/tasks", func(tasks chi.Router) {
				tasks.Get("/", taskHandler.List)
				tasks.Post("/", taskHandler.Create)
				tasks.Get("/{taskId}", taskHandler.Detail)
			})
		})

		api.Route("/agent", func(agent chi.Router) {
			agent.Post("/heartbeat", agentHandler.Heartbeat)
			agent.Get("/login-tasks/{deviceCode}", agentHandler.ListLoginTasks)
			agent.Post("/login-sessions/{sessionId}/event", agentHandler.PushLoginEvent)
			agent.Get("/login-sessions/{sessionId}/actions", agentHandler.ListLoginActions)
			agent.Get("/publish-tasks/{deviceCode}", agentHandler.ListPublishTasks)
			agent.Post("/publish-tasks/sync", agentHandler.SyncPublishTask)
		})
	})

	return r
}
