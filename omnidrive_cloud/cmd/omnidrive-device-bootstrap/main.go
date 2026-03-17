package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"omnidrive_cloud/internal/config"
	"omnidrive_cloud/internal/database"
	"omnidrive_cloud/internal/logging"
	"omnidrive_cloud/internal/security"
	"omnidrive_cloud/internal/store"
)

type bootstrapOptions struct {
	deviceCode    string
	deviceName    string
	agentKey      string
	ownerEmail    string
	ownerName     string
	ownerPassword string
	chatModel     string
	imageModel    string
	videoModel    string
}

func main() {
	opts := parseFlags()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := config.Load()
	logger := logging.New(cfg)
	db, err := database.New(ctx, cfg, logger)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	repo := store.New(db.Pool)
	passwords := security.NewTokenManager("bootstrap-only", 60)

	defaultChatModel, defaultImageModel, defaultVideoModel, err := loadEffectiveDefaults(ctx, cfg, repo)
	if err != nil {
		log.Fatalf("load effective model defaults: %v", err)
	}
	if opts.chatModel == "" {
		opts.chatModel = defaultChatModel
	}
	if opts.imageModel == "" {
		opts.imageModel = defaultImageModel
	}
	if opts.videoModel == "" {
		opts.videoModel = defaultVideoModel
	}

	user, createdUser, err := ensureOwnerUser(ctx, repo, passwords, opts)
	if err != nil {
		log.Fatalf("ensure owner user: %v", err)
	}

	existingDevice, err := repo.GetDeviceByCode(ctx, opts.deviceCode)
	if err != nil {
		log.Fatalf("load device: %v", err)
	}
	if existingDevice != nil && strings.TrimSpace(existingDevice.GetAgentKey()) != "" && existingDevice.GetAgentKey() != opts.agentKey {
		log.Fatalf("device %s already exists with a different agent key; please unbind or align the configured agent key first", opts.deviceCode)
	}
	if existingDevice != nil && existingDevice.OwnerUserID != nil && *existingDevice.OwnerUserID != user.User.ID {
		log.Fatalf("device %s is already claimed by another user", opts.deviceCode)
	}

	runtimePayload, err := json.Marshal(map[string]any{
		"bootstrapSource": "factory_device_bootstrap",
		"bootstrappedAt":  time.Now().UTC().Format(time.RFC3339),
		"ownerEmail":      user.User.Email,
	})
	if err != nil {
		log.Fatalf("marshal runtime payload: %v", err)
	}

	device, err := repo.UpsertHeartbeatDevice(ctx, store.HeartbeatInput{
		DeviceCode:     opts.deviceCode,
		AgentKey:       opts.agentKey,
		DeviceName:     opts.deviceName,
		RuntimePayload: runtimePayload,
	})
	if err != nil {
		log.Fatalf("upsert device: %v", err)
	}

	if device.OwnerUserID == nil || *device.OwnerUserID != user.User.ID {
		device, err = repo.ClaimDevice(ctx, opts.deviceCode, user.User.ID)
		if err != nil {
			log.Fatalf("claim device: %v", err)
		}
		if device == nil {
			log.Fatalf("claim device returned nil for %s", opts.deviceCode)
		}
	}

	reasoningModel := opts.chatModel
	enabled := true
	device, err = repo.UpdateDevice(ctx, device.ID, user.User.ID, store.UpdateDeviceInput{
		Name:                  stringPtr(opts.deviceName),
		DefaultReasoningModel: &reasoningModel,
		DefaultChatModel:      &opts.chatModel,
		DefaultImageModel:     &opts.imageModel,
		DefaultVideoModel:     &opts.videoModel,
		IsEnabled:             &enabled,
	})
	if err != nil {
		log.Fatalf("update device defaults: %v", err)
	}
	if device == nil {
		log.Fatalf("update device returned nil for %s", opts.deviceCode)
	}

	result := map[string]any{
		"user": map[string]any{
			"id":        user.User.ID,
			"email":     user.User.Email,
			"name":      user.User.Name,
			"isActive":  user.User.IsActive,
			"isCreated": createdUser,
		},
		"device": map[string]any{
			"id":                    device.ID,
			"deviceCode":            device.DeviceCode,
			"name":                  device.Name,
			"isEnabled":             device.IsEnabled,
			"defaultReasoningModel": valueOrEmpty(device.DefaultReasoningModel),
			"defaultChatModel":      valueOrEmpty(device.DefaultChatModel),
			"defaultImageModel":     valueOrEmpty(device.DefaultImageModel),
			"defaultVideoModel":     valueOrEmpty(device.DefaultVideoModel),
			"ownerUserId":           valueOrEmpty(device.OwnerUserID),
			"agentKeyConfigured":    strings.TrimSpace(device.GetAgentKey()) != "",
		},
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("marshal result: %v", err)
	}
	fmt.Println(string(encoded))
}

func parseFlags() bootstrapOptions {
	var opts bootstrapOptions
	flag.StringVar(&opts.deviceCode, "device-code", "", "OmniBull device code (MAC-based code)")
	flag.StringVar(&opts.deviceName, "device-name", "A001", "Display name stored for the OmniBull device")
	flag.StringVar(&opts.agentKey, "agent-key", "", "Agent key used by the OmniBull bridge")
	flag.StringVar(&opts.ownerEmail, "owner-email", "", "Existing OmniDrive user email to bind the device to")
	flag.StringVar(&opts.ownerName, "owner-name", "", "Display name used when creating a missing OmniDrive user")
	flag.StringVar(&opts.ownerPassword, "owner-password", "", "Password used when creating a missing OmniDrive user")
	flag.StringVar(&opts.chatModel, "chat-model", "", "Override default chat model")
	flag.StringVar(&opts.imageModel, "image-model", "", "Override default image model")
	flag.StringVar(&opts.videoModel, "video-model", "", "Override default video model")
	flag.Parse()

	opts.deviceCode = strings.TrimSpace(opts.deviceCode)
	opts.deviceName = strings.TrimSpace(opts.deviceName)
	opts.agentKey = strings.TrimSpace(opts.agentKey)
	opts.ownerEmail = strings.TrimSpace(strings.ToLower(opts.ownerEmail))
	opts.ownerName = strings.TrimSpace(opts.ownerName)
	opts.ownerPassword = strings.TrimSpace(opts.ownerPassword)
	opts.chatModel = strings.TrimSpace(opts.chatModel)
	opts.imageModel = strings.TrimSpace(opts.imageModel)
	opts.videoModel = strings.TrimSpace(opts.videoModel)

	if opts.deviceCode == "" {
		log.Fatal("--device-code is required")
	}
	if opts.agentKey == "" {
		log.Fatal("--agent-key is required")
	}
	if opts.ownerEmail == "" {
		log.Fatal("--owner-email is required")
	}
	if opts.deviceName == "" {
		opts.deviceName = "A001"
	}

	return opts
}

func ensureOwnerUser(ctx context.Context, repo *store.Store, passwords *security.TokenManager, opts bootstrapOptions) (*store.UserWithPassword, bool, error) {
	existing, err := repo.GetUserByEmail(ctx, opts.ownerEmail)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		if !existing.User.IsActive {
			return nil, false, fmt.Errorf("owner user %s is inactive", opts.ownerEmail)
		}
		return existing, false, nil
	}
	if opts.ownerPassword == "" {
		return nil, false, fmt.Errorf("owner user %s does not exist; --owner-password is required to create it", opts.ownerEmail)
	}
	name := opts.ownerName
	if name == "" {
		name = defaultOwnerName(opts.ownerEmail)
	}
	passwordHash, err := passwords.HashPassword(opts.ownerPassword)
	if err != nil {
		return nil, false, err
	}
	user, err := repo.CreateUser(ctx, store.CreateUserInput{
		ID:           uuid.NewString(),
		Email:        opts.ownerEmail,
		Name:         name,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return nil, false, err
	}
	return &store.UserWithPassword{User: *user, PasswordHash: passwordHash}, true, nil
}

func loadEffectiveDefaults(ctx context.Context, cfg config.Config, repo *store.Store) (string, string, string, error) {
	chatModel := strings.TrimSpace(cfg.DefaultChatModel)
	imageModel := strings.TrimSpace(cfg.DefaultImageModel)
	videoModel := strings.TrimSpace(cfg.DefaultVideoModel)

	record, err := repo.GetAdminSystemSettings(ctx)
	if err != nil {
		return "", "", "", err
	}
	if record != nil {
		if value := strings.TrimSpace(record.DefaultChatModel); value != "" {
			chatModel = value
		}
		if value := strings.TrimSpace(record.DefaultImageModel); value != "" {
			imageModel = value
		}
		if value := strings.TrimSpace(record.DefaultVideoModel); value != "" {
			videoModel = value
		}
	}

	return chatModel, imageModel, videoModel, nil
}

func defaultOwnerName(email string) string {
	name := strings.TrimSpace(email)
	if index := strings.Index(name, "@"); index > 0 {
		name = name[:index]
	}
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.TrimSpace(name)
	if name == "" {
		return "OmniDrive User"
	}
	return name
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
