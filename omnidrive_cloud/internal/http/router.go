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
	overviewHandler := handlers.NewOverviewHandler(app)
	deviceHandler := handlers.NewDeviceHandler(app)
	accountHandler := handlers.NewAccountHandler(app)
	materialHandler := handlers.NewMaterialHandler(app)
	skillHandler := handlers.NewSkillHandler(app)
	taskHandler := handlers.NewTaskHandler(app)
	aiHandler := handlers.NewAIHandler(app)
	billingHandler := handlers.NewBillingHandler(app)
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

			private.Route("/overview", func(overview chi.Router) {
				overview.Get("/summary", overviewHandler.Summary)
			})

			private.Get("/history", overviewHandler.History)

			private.Route("/devices", func(devices chi.Router) {
				devices.Get("/", deviceHandler.List)
				devices.Post("/claim", deviceHandler.Claim)
				devices.Get("/{deviceId}", deviceHandler.Detail)
				devices.Patch("/{deviceId}", deviceHandler.Update)
			})

			private.Route("/materials", func(materials chi.Router) {
				materials.Get("/roots", materialHandler.Roots)
				materials.Get("/list", materialHandler.List)
				materials.Get("/file", materialHandler.File)
			})

			private.Route("/accounts", func(accounts chi.Router) {
				accounts.Get("/", accountHandler.List)
				accounts.Get("/{accountId}", accountHandler.Detail)
				accounts.Delete("/{accountId}", accountHandler.Delete)
				accounts.Post("/{accountId}/validate", accountHandler.Validate)
				accounts.Post("/remote-login", accountHandler.CreateRemoteLogin)
				accounts.Get("/login-sessions/{sessionId}", accountHandler.GetLoginSession)
				accounts.Post("/login-sessions/{sessionId}/actions", accountHandler.CreateLoginAction)
			})

			private.Route("/skills", func(skills chi.Router) {
				skills.Get("/", skillHandler.List)
				skills.Post("/", skillHandler.Create)
				skills.Get("/{skillId}", skillHandler.Detail)
				skills.Patch("/{skillId}", skillHandler.Update)
				skills.Delete("/{skillId}", skillHandler.Delete)
				skills.Get("/{skillId}/assets", skillHandler.ListAssets)
				skills.Post("/{skillId}/assets", skillHandler.CreateAsset)
				skills.Post("/{skillId}/upload", skillHandler.UploadAsset)
			})

			private.Route("/tasks", func(tasks chi.Router) {
				tasks.Get("/", taskHandler.List)
				tasks.Post("/", taskHandler.Create)
				tasks.Get("/{taskId}", taskHandler.Detail)
				tasks.Get("/{taskId}/events", taskHandler.Events)
				tasks.Get("/{taskId}/artifacts", taskHandler.Artifacts)
				tasks.Get("/{taskId}/materials", taskHandler.Materials)
				tasks.Post("/{taskId}/cancel", taskHandler.Cancel)
				tasks.Post("/{taskId}/retry", taskHandler.Retry)
				tasks.Patch("/{taskId}", taskHandler.Update)
				tasks.Delete("/{taskId}", taskHandler.Delete)
			})

			private.Route("/ai", func(ai chi.Router) {
				ai.Get("/models", aiHandler.ListModels)
				ai.Get("/jobs", aiHandler.ListJobs)
				ai.Post("/jobs", aiHandler.CreateJob)
				ai.Get("/jobs/{jobId}", aiHandler.DetailJob)
			})

			private.Route("/billing", func(billing chi.Router) {
				billing.Get("/packages", billingHandler.ListPackages)
				billing.Get("/ledger", billingHandler.Ledger)
			})
		})

		api.Route("/agent", func(agent chi.Router) {
			agent.Post("/heartbeat", agentHandler.Heartbeat)
			agent.Post("/accounts/sync", agentHandler.SyncAccount)
			agent.Post("/materials/roots/sync", materialHandler.SyncRoots)
			agent.Post("/materials/directory/sync", materialHandler.SyncDirectory)
			agent.Post("/materials/file/sync", materialHandler.SyncFile)
			agent.Get("/login-tasks/{deviceCode}", agentHandler.ListLoginTasks)
			agent.Post("/login-sessions/{sessionId}/event", agentHandler.PushLoginEvent)
			agent.Get("/login-sessions/{sessionId}/actions", agentHandler.ListLoginActions)
			agent.Get("/publish-tasks/{deviceCode}", agentHandler.ListPublishTasks)
			agent.Post("/publish-tasks/{taskId}/claim", agentHandler.ClaimPublishTask)
			agent.Post("/publish-tasks/{taskId}/renew", agentHandler.RenewPublishTaskLease)
			agent.Post("/publish-tasks/sync", agentHandler.SyncPublishTask)
		})
	})

	return r
}
