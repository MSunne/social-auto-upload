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
	adminAuthHandler := handlers.NewAdminAuthHandler(app)
	adminConsoleHandler := handlers.NewAdminConsoleHandler(app)

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
				devices.Get("/{deviceId}/workspace", deviceHandler.Workspace)
				devices.Patch("/{deviceId}", deviceHandler.Update)
			})

			private.Route("/materials", func(materials chi.Router) {
				materials.Get("/roots", materialHandler.Roots)
				materials.Get("/list", materialHandler.List)
				materials.Get("/file", materialHandler.File)
				materials.Get("/workspace", materialHandler.Workspace)
			})

			private.Route("/accounts", func(accounts chi.Router) {
				accounts.Get("/", accountHandler.List)
				accounts.Get("/{accountId}", accountHandler.Detail)
				accounts.Get("/{accountId}/workspace", accountHandler.Workspace)
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
				skills.Get("/{skillId}/workspace", skillHandler.Workspace)
				skills.Get("/{skillId}/impact", skillHandler.Impact)
				skills.Patch("/{skillId}", skillHandler.Update)
				skills.Delete("/{skillId}", skillHandler.Delete)
				skills.Get("/{skillId}/assets", skillHandler.ListAssets)
				skills.Post("/{skillId}/assets", skillHandler.CreateAsset)
				skills.Post("/{skillId}/upload", skillHandler.UploadAsset)
			})

			private.Route("/tasks", func(tasks chi.Router) {
				tasks.Get("/", taskHandler.List)
				tasks.Get("/diagnostics", taskHandler.Diagnostics)
				tasks.Post("/bulk-repair", taskHandler.BulkRepair)
				tasks.Post("/bulk-action", taskHandler.BulkAction)
				tasks.Post("/", taskHandler.Create)
				tasks.Get("/{taskId}", taskHandler.Detail)
				tasks.Get("/{taskId}/workspace", taskHandler.Workspace)
				tasks.Get("/{taskId}/events", taskHandler.Events)
				tasks.Get("/{taskId}/artifacts", taskHandler.Artifacts)
				tasks.Get("/{taskId}/materials", taskHandler.Materials)
				tasks.Post("/{taskId}/refresh-materials", taskHandler.RefreshMaterials)
				tasks.Post("/{taskId}/refresh-skill", taskHandler.RefreshSkill)
				tasks.Post("/{taskId}/cancel", taskHandler.Cancel)
				tasks.Post("/{taskId}/force-release", taskHandler.ForceRelease)
				tasks.Post("/{taskId}/resume", taskHandler.Resume)
				tasks.Post("/{taskId}/manual-resolve", taskHandler.ManualResolve)
				tasks.Post("/{taskId}/retry", taskHandler.Retry)
				tasks.Patch("/{taskId}", taskHandler.Update)
				tasks.Delete("/{taskId}", taskHandler.Delete)
			})

			private.Route("/ai", func(ai chi.Router) {
				ai.Get("/models", aiHandler.ListModels)
				ai.Get("/jobs", aiHandler.ListJobs)
				ai.Post("/jobs", aiHandler.CreateJob)
				ai.Get("/jobs/{jobId}", aiHandler.DetailJob)
				ai.Get("/jobs/{jobId}/workspace", aiHandler.WorkspaceJob)
				ai.Get("/jobs/{jobId}/artifacts", aiHandler.ListArtifacts)
				ai.Post("/jobs/{jobId}/artifacts/upload", aiHandler.UploadArtifact)
				ai.Post("/jobs/{jobId}/publish-task", aiHandler.CreatePublishTask)
				ai.Patch("/jobs/{jobId}", aiHandler.UpdateJob)
				ai.Post("/jobs/{jobId}/cancel", aiHandler.CancelJob)
				ai.Post("/jobs/{jobId}/retry", aiHandler.RetryJob)
				ai.Post("/jobs/{jobId}/force-release", aiHandler.ForceReleaseJob)
			})

			private.Route("/billing", func(billing chi.Router) {
				billing.Get("/summary", billingHandler.Summary)
				billing.Get("/packages", billingHandler.ListPackages)
				billing.Get("/rules", billingHandler.ListPricingRules)
				billing.Get("/ledger", billingHandler.Ledger)
				billing.Get("/orders", billingHandler.ListOrders)
				billing.Post("/orders", billingHandler.CreateOrder)
				billing.Get("/orders/{orderId}", billingHandler.DetailOrder)
				billing.Get("/orders/{orderId}/events", billingHandler.ListOrderEvents)
				billing.Post("/orders/{orderId}/manual-submit", billingHandler.SubmitManualRecharge)
			})
		})

		api.Route("/agent", func(agent chi.Router) {
			agent.Post("/heartbeat", agentHandler.Heartbeat)
			agent.Post("/accounts/sync", agentHandler.SyncAccount)
			agent.Get("/ai-jobs/{deviceCode}", agentHandler.ListAIJobs)
			agent.Post("/ai-jobs/sync", agentHandler.SyncAIJob)
			agent.Post("/ai-jobs/{jobId}/delivery", agentHandler.UpdateAIJobDelivery)
			agent.Get("/skills/{deviceCode}", agentHandler.ListSkills)
			agent.Post("/skills/sync", agentHandler.SyncSkillStates)
			agent.Post("/skills/retired-ack", agentHandler.AckRetiredSkills)
			agent.Post("/materials/roots/sync", materialHandler.SyncRoots)
			agent.Post("/materials/directory/sync", materialHandler.SyncDirectory)
			agent.Post("/materials/file/sync", materialHandler.SyncFile)
			agent.Get("/login-tasks/{deviceCode}", agentHandler.ListLoginTasks)
			agent.Post("/login-sessions/{sessionId}/event", agentHandler.PushLoginEvent)
			agent.Get("/login-sessions/{sessionId}/actions", agentHandler.ListLoginActions)
			agent.Get("/publish-tasks/{deviceCode}", agentHandler.ListPublishTasks)
			agent.Get("/publish-tasks/{taskId}/package", agentHandler.PublishTaskPackage)
			agent.Post("/publish-tasks/{taskId}/claim", agentHandler.ClaimPublishTask)
			agent.Post("/publish-tasks/{taskId}/renew", agentHandler.RenewPublishTaskLease)
			agent.Post("/publish-tasks/{taskId}/release", agentHandler.ReleasePublishTaskLease)
			agent.Post("/publish-tasks/sync", agentHandler.SyncPublishTask)
		})
	})

	r.Route("/api/admin/v1", func(admin chi.Router) {
		admin.Route("/auth", func(auth chi.Router) {
			auth.Post("/login", adminAuthHandler.Login)
		})

		admin.Group(func(private chi.Router) {
			private.Use(authmiddleware.RequireAdmin(app))

			private.Get("/me", adminAuthHandler.Me)
			private.Get("/admins", adminAuthHandler.ListAdmins)
			private.Get("/system-config", adminAuthHandler.SystemConfig)

			private.With(authmiddleware.RequireAdminPermission("admin.manage")).Get("/dashboard/summary", adminConsoleHandler.DashboardSummary)
			private.With(authmiddleware.RequireAdminPermission("user.read")).Get("/users", adminConsoleHandler.ListUsers)
			private.With(authmiddleware.RequireAdminPermission("device.read")).Get("/devices", adminConsoleHandler.ListDevices)
			private.With(authmiddleware.RequireAdminPermission("finance.read")).Get("/pricing/packages", adminConsoleHandler.ListPricingPackages)
			private.With(authmiddleware.RequireAdminPermission("finance.read")).Get("/pricing/rules", adminConsoleHandler.ListPricingRules)
			private.With(authmiddleware.RequireAdminPermission("finance.read")).Get("/orders", adminConsoleHandler.ListOrders)
			private.With(authmiddleware.RequireAdminPermission("finance.read")).Get("/wallet-ledgers", adminConsoleHandler.ListWalletLedgers)
			private.With(authmiddleware.RequireAdminPermission("support_recharge.review")).Get("/support-recharges", adminConsoleHandler.ListSupportRecharges)
			private.With(authmiddleware.RequireAdminPermission("support_recharge.review")).Get("/support-recharges/{orderId}", adminConsoleHandler.DetailSupportRecharge)
			private.With(authmiddleware.RequireAdminPermission("support_recharge.review")).Get("/support-recharges/{orderId}/events", adminConsoleHandler.ListSupportRechargeEvents)
			private.With(authmiddleware.RequireAdminPermission("support_recharge.review")).Post("/support-recharges/{orderId}/credit", adminConsoleHandler.CreditSupportRecharge)
			private.With(authmiddleware.RequireAdminPermission("support_recharge.review")).Post("/support-recharges/{orderId}/reject", adminConsoleHandler.RejectSupportRecharge)
			private.With(authmiddleware.RequireAdminPermission("distribution.read")).Get("/distribution/relations", adminConsoleHandler.ListDistributionRelations)
			private.With(authmiddleware.RequireAdminPermission("distribution.read")).Get("/distribution/commissions", adminConsoleHandler.ListDistributionCommissions)
			private.With(authmiddleware.RequireAdminPermission("distribution.read")).Get("/distribution/settlements", adminConsoleHandler.ListDistributionSettlements)
			private.With(authmiddleware.RequireAdminPermission("withdrawal.review")).Get("/withdrawals", adminConsoleHandler.ListWithdrawals)
			private.With(authmiddleware.RequireAdminPermission("admin.manage")).Get("/audits", adminConsoleHandler.ListAudits)
		})
	})

	return r
}
