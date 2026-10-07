package enginehub

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"dockvista/internal/adapters/broker"
	dockeradapter "dockvista/internal/adapters/docker"
	"dockvista/internal/adapters/envstore"
	"dockvista/internal/adapters/gitclone"
	"dockvista/internal/adapters/httpapi"
	"dockvista/internal/adapters/regstore"
	"dockvista/internal/adapters/stackstore"
	"dockvista/internal/adapters/store"
	"dockvista/internal/core/domain"
	"dockvista/internal/core/service"
)

type runtime struct {
	meta       domain.Environment
	docker     *dockeradapter.Client
	containers *service.ContainerService
	images     *service.ImageService
	volumes    *service.VolumeService
	networks   *service.NetworkService
	system     *service.SystemService
	storage    *service.StorageService
	stacks     *service.StackService
	swarm      *service.SwarmService
	events     *broker.Broker[domain.Event]
	cancel     context.CancelFunc
}

// Hub owns one live Docker runtime per named environment.
type Hub struct {
	mu       sync.RWMutex
	runtimes map[string]*runtime
	envs     *envstore.Store
	stacks   *stackstore.Store
	regs     *regstore.Store
	log      *slog.Logger
	poll     time.Duration
	parent   context.Context
}

func Start(parent context.Context, dataDir string, poll time.Duration, log *slog.Logger) (*Hub, error) {
	envs, err := envstore.New(dataDir)
	if err != nil {
		return nil, err
	}
	stacks, err := stackstore.New(dataDir)
	if err != nil {
		return nil, err
	}
	regs, err := regstore.New(dataDir)
	if err != nil {
		return nil, err
	}
	h := &Hub{
		runtimes: map[string]*runtime{},
		envs:     envs,
		stacks:   stacks,
		regs:     regs,
		log:      log,
		poll:     poll,
		parent:   parent,
	}
	local, err := dockeradapter.New()
	if err != nil {
		return nil, err
	}
	h.attach(domain.Environment{
		ID:   domain.LocalEnvironmentID,
		Name: "Local",
		Kind: domain.EnvKindLocal,
		Host: os.Getenv("DOCKER_HOST"),
	}, local)

	saved, err := envs.List()
	if err != nil {
		return nil, err
	}
	for _, env := range saved {
		if err := h.startTCP(env); err != nil {
			log.Warn("environment offline", "id", env.ID, "name", env.Name, "error", err)
		}
	}
	return h, nil
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, rt := range h.runtimes {
		rt.cancel()
		_ = rt.docker.Close()
	}
}

func (h *Hub) LocalSystem() *service.SystemService {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if rt := h.runtimes[domain.LocalEnvironmentID]; rt != nil {
		return rt.system
	}
	return nil
}

func (h *Hub) Resolve(_ context.Context, id string) (httpapi.BoundEngine, error) {
	if id == "" {
		id = domain.LocalEnvironmentID
	}
	h.mu.RLock()
	rt := h.runtimes[id]
	h.mu.RUnlock()
	if rt == nil {
		return httpapi.BoundEngine{}, domain.ErrNotFound
	}
	info, err := rt.docker.Ping(context.Background())
	rt.meta.Reachable = err == nil && info.Reachable
	rt.meta.Version = info.Version
	return httpapi.BoundEngine{
		Containers: rt.containers,
		Images:     rt.images,
		Volumes:    rt.volumes,
		Networks:   rt.networks,
		System:     rt.system,
		Storage:    rt.storage,
		Events:     rt.events,
		Stacks:     rt.stacks,
		Swarm:      rt.swarm,
	}, nil
}

func (h *Hub) List(_ context.Context) ([]domain.Environment, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]domain.Environment, 0, len(h.runtimes))
	if rt := h.runtimes[domain.LocalEnvironmentID]; rt != nil {
		meta := rt.meta
		info, err := rt.docker.Ping(context.Background())
		meta.Reachable = err == nil && info.Reachable
		meta.Version = info.Version
		meta.Host = displayHost(meta.Host)
		out = append(out, meta)
	}
	saved, err := h.envs.List()
	if err != nil {
		return nil, err
	}
	for _, env := range saved {
		if rt := h.runtimes[env.ID]; rt != nil {
			info, err := rt.docker.Ping(context.Background())
			env.Reachable = err == nil && info.Reachable
			env.Version = info.Version
		}
		out = append(out, env)
	}
	return out, nil
}

func (h *Hub) Create(ctx context.Context, spec domain.EnvironmentSpec) (domain.Environment, error) {
	if err := domain.ValidateEnvironmentName(spec.Name); err != nil {
		return domain.Environment{}, err
	}
	kind, err := domain.ParseEnvironmentKind(string(spec.Kind))
	if err != nil {
		return domain.Environment{}, err
	}
	spec.Kind = kind
	if err := domain.ValidateTCPHost(spec.Host); err != nil {
		return domain.Environment{}, err
	}
	if spec.TLSCA == "" || spec.TLSCert == "" || spec.TLSKey == "" {
		return domain.Environment{}, fmt.Errorf("%w: TCP environments require ca, cert, and key PEM", domain.ErrInvalidInput)
	}
	env, err := h.envs.Create(spec)
	if err != nil {
		return domain.Environment{}, err
	}
	if err := h.startTCP(env); err != nil {
		_ = h.envs.Delete(env.ID)
		return domain.Environment{}, err
	}
	h.mu.RLock()
	rt := h.runtimes[env.ID]
	h.mu.RUnlock()
	if rt == nil {
		_ = h.envs.Delete(env.ID)
		return domain.Environment{}, domain.ErrEngine
	}
	info, err := rt.docker.Ping(ctx)
	if err != nil {
		h.removeRuntime(env.ID)
		_ = h.envs.Delete(env.ID)
		return domain.Environment{}, err
	}
	env.Reachable = info.Reachable
	env.Version = info.Version
	return env, nil
}

func (h *Hub) Delete(_ context.Context, id string) error {
	if id == domain.LocalEnvironmentID {
		return domain.ErrLocalProtected
	}
	h.removeRuntime(id)
	return h.envs.Delete(id)
}

func (h *Hub) Registries() *regstore.Store { return h.regs }

func (h *Hub) startTCP(env domain.Environment) error {
	ca, cert, key := h.envs.TLSPaths(env.ID)
	cli, err := dockeradapter.NewTCP(env.Host, ca, cert, key)
	if err != nil {
		return err
	}
	h.attach(env, cli)
	return nil
}

func (h *Hub) attach(meta domain.Environment, cli *dockeradapter.Client) {
	ctx, cancel := context.WithCancel(h.parent)
	sys := service.NewSystemService(cli)
	images := service.NewImageService(cli)
	images.SetRegistryAuth(h.authHeader)
	containers := service.New(cli, store.New(), h.log)
	volumes := service.NewVolumeService(cli)
	networks := service.NewNetworkService(cli)
	stacks := service.NewStackService(h.stacks, cli, gitclone.New())
	stacks.SetRegistryAuth(h.authHeader)
	events := broker.New[domain.Event]()
	rt := &runtime{
		meta:       meta,
		docker:     cli,
		containers: containers,
		images:     images,
		volumes:    volumes,
		networks:   networks,
		system:     sys,
		storage:    service.NewStorageService(cli, sys.InvalidateDiskUsage),
		stacks:     stacks,
		swarm:      service.NewSwarmService(cli),
		events:     events,
		cancel:     cancel,
	}
	h.mu.Lock()
	h.runtimes[meta.ID] = rt
	h.mu.Unlock()
	go containers.RunCollector(ctx, h.poll)
	go bridgeDockerEvents(ctx, cli, containers, events, h.log)
}

func (h *Hub) removeRuntime(id string) {
	h.mu.Lock()
	rt := h.runtimes[id]
	delete(h.runtimes, id)
	h.mu.Unlock()
	if rt == nil {
		return
	}
	rt.cancel()
	_ = rt.docker.Close()
}

func (h *Hub) authHeader(ref string) string {
	user, pass, ok := h.regs.AuthForImage(ref)
	if !ok {
		return ""
	}
	return encodeRegistryAuth(user, pass, domain.RegistryHostFromImage(ref))
}

func displayHost(host string) string {
	if host == "" {
		return "local socket"
	}
	return host
}
