package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dockvista/internal/core/domain"
)

type StackStore interface {
	List() ([]domain.Stack, error)
	Get(id string) (domain.Stack, error)
	Create(name, yamlBody string) (domain.Stack, error)
	CreateGit(name, gitURL, gitRef, composeFile string) (domain.Stack, error)
	UpdateYAML(id, yamlBody string) (domain.Stack, error)
	SetComposeFile(id, rel string) error
	Reload(id string) (domain.Stack, error)
	SetGitAuth(id, username, token string) error
	GitAuth(id string) (username, token string)
	Delete(id string) error
	Dir(id string) string
}

type GitCloner interface {
	Clone(ctx context.Context, dir, remote, ref, username, token string) error
	Update(ctx context.Context, dir, remote, ref, username, token string) error
}

type stackEngine interface {
	ListContainers(ctx context.Context) ([]domain.Container, error)
	CreateContainer(ctx context.Context, spec domain.ContainerSpec) (id string, started bool, err error)
	RemoveContainer(ctx context.Context, id string, force bool) error
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	PullImage(ctx context.Context, ref, registryAuth string) (io.ReadCloser, error)
	CreateVolume(ctx context.Context, name, driver string, labels map[string]string) (domain.Volume, error)
	RemoveVolume(ctx context.Context, name string, force bool) error
	CreateNetwork(ctx context.Context, name, driver string, labels map[string]string) (domain.Network, error)
	RemoveNetwork(ctx context.Context, id string) error
	ConnectNetwork(ctx context.Context, networkID, containerID string) error
}

// StackService stores compose files in a DockVista-owned workspace and
// applies them through the Docker API of the active environment. No
// compose CLI, no host homedir.
type StackService struct {
	store        StackStore
	docker       stackEngine
	git          GitCloner
	registryAuth func(ref string) string
}

func NewStackService(store StackStore, docker stackEngine, git GitCloner) *StackService {
	return &StackService{store: store, docker: docker, git: git}
}

func (s *StackService) SetRegistryAuth(fn func(ref string) string) {
	s.registryAuth = fn
}

func (s *StackService) List() ([]domain.Stack, error) {
	return s.store.List()
}

func (s *StackService) Get(id string) (domain.Stack, error) {
	return s.store.Get(id)
}

func (s *StackService) Create(name, yamlBody string) (domain.Stack, error) {
	if err := domain.ValidateStackName(name); err != nil {
		return domain.Stack{}, err
	}
	if len(yamlBody) == 0 || len(yamlBody) > domain.MaxStackYAML {
		return domain.Stack{}, domain.ErrInvalidInput
	}
	if _, err := parseComposeYAML(yamlBody, "."); err != nil {
		return domain.Stack{}, err
	}
	return s.store.Create(name, yamlBody)
}

func (s *StackService) Update(id, yamlBody string) (domain.Stack, error) {
	st, err := s.store.Get(id)
	if err != nil {
		return domain.Stack{}, err
	}
	if st.GitURL != "" {
		return domain.Stack{}, fmt.Errorf("%w: git stacks are updated with sync, not pasted YAML", domain.ErrInvalidInput)
	}
	if len(yamlBody) == 0 || len(yamlBody) > domain.MaxStackYAML {
		return domain.Stack{}, domain.ErrInvalidInput
	}
	if _, err := parseComposeYAML(yamlBody, "."); err != nil {
		return domain.Stack{}, err
	}
	return s.store.UpdateYAML(id, yamlBody)
}

func (s *StackService) CreateFromGit(ctx context.Context, name, gitURL, gitRef, composeFile, username, token string) (domain.Stack, error) {
	if err := domain.ValidateStackName(name); err != nil {
		return domain.Stack{}, err
	}
	if err := domain.ValidateGitRemote(gitURL); err != nil {
		return domain.Stack{}, err
	}
	ref, err := domain.NormalizeGitRef(gitRef)
	if err != nil {
		return domain.Stack{}, err
	}
	if err := domain.ValidateComposeRel(composeFile); err != nil {
		return domain.Stack{}, err
	}
	if s.git == nil {
		return domain.Stack{}, domain.ErrGitClone
	}
	st, err := s.store.CreateGit(name, gitURL, ref, strings.TrimSpace(composeFile))
	if err != nil {
		return domain.Stack{}, err
	}
	if err := s.store.SetGitAuth(st.ID, username, token); err != nil {
		_ = s.store.Delete(st.ID)
		return domain.Stack{}, err
	}
	if err := s.git.Clone(ctx, s.store.Dir(st.ID), gitURL, ref, username, token); err != nil {
		_ = s.store.Delete(st.ID)
		return domain.Stack{}, err
	}
	rel, err := findComposeFile(s.store.Dir(st.ID), composeFile)
	if err != nil {
		_ = s.store.Delete(st.ID)
		return domain.Stack{}, err
	}
	if err := s.store.SetComposeFile(st.ID, rel); err != nil {
		_ = s.store.Delete(st.ID)
		return domain.Stack{}, err
	}
	st, err = s.store.Reload(st.ID)
	if err != nil {
		_ = s.store.Delete(st.ID)
		return domain.Stack{}, err
	}
	if _, err := parseComposeYAMLAt(st.YAML, composeDir(s.store.Dir(st.ID), rel), s.store.Dir(st.ID)); err != nil {
		_ = s.store.Delete(st.ID)
		return domain.Stack{}, err
	}
	return st, nil
}

func (s *StackService) Sync(ctx context.Context, id string) (domain.Stack, error) {
	st, err := s.store.Get(id)
	if err != nil {
		return domain.Stack{}, err
	}
	if st.GitURL == "" {
		return domain.Stack{}, fmt.Errorf("%w: not a git stack", domain.ErrInvalidInput)
	}
	if s.git == nil {
		return domain.Stack{}, domain.ErrGitClone
	}
	user, token := s.store.GitAuth(id)
	if err := s.git.Update(ctx, s.store.Dir(id), st.GitURL, st.GitRef, user, token); err != nil {
		return domain.Stack{}, err
	}
	rel, err := findComposeFile(s.store.Dir(id), st.ComposeFile)
	if err != nil {
		return domain.Stack{}, err
	}
	if rel != st.ComposeFile {
		if err := s.store.SetComposeFile(id, rel); err != nil {
			return domain.Stack{}, err
		}
	}
	st, err = s.store.Reload(id)
	if err != nil {
		return domain.Stack{}, err
	}
	if _, err := parseComposeYAMLAt(st.YAML, composeDir(s.store.Dir(id), rel), s.store.Dir(id)); err != nil {
		return domain.Stack{}, err
	}
	return st, nil
}

func findComposeFile(root, specified string) (string, error) {
	specified = strings.TrimSpace(strings.ReplaceAll(specified, "\\", "/"))
	if specified != "" {
		if err := domain.ValidateComposeRel(specified); err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(specified))); err != nil {
			return "", fmt.Errorf("%w: compose file %s not in the clone", domain.ErrNotFound, specified)
		}
		return specified, nil
	}
	for _, name := range []string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("%w: no compose.yml at the repo root", domain.ErrInvalidInput)
}

func composeDir(root, rel string) string {
	if rel == "" || filepath.Dir(filepath.FromSlash(rel)) == "." {
		return root
	}
	return filepath.Join(root, filepath.Dir(filepath.FromSlash(rel)))
}

func (s *StackService) Delete(id string) error {
	return s.store.Delete(id)
}

func (s *StackService) Up(ctx context.Context, id string) error {
	st, err := s.store.Get(id)
	if err != nil {
		return err
	}
	rel := st.ComposeFile
	parsed, err := parseComposeYAMLAt(st.YAML, composeDir(s.store.Dir(id), rel), s.store.Dir(id))
	if err != nil {
		return err
	}
	project := domain.ProjectNameFor(st.Name)
	labels := map[string]string{
		domain.ComposeProjectKey: project,
		domain.StackLabelKey:     st.ID,
	}
	netIDs := map[string]string{}
	if len(parsed.Networks) == 0 {
		n, err := s.docker.CreateNetwork(ctx, project+"_default", "bridge", labels)
		if err != nil && !alreadyExists(err) {
			return err
		}
		if err == nil {
			netIDs["default"] = n.ID
		}
	}
	for _, net := range parsed.Networks {
		n, err := s.docker.CreateNetwork(ctx, project+"_"+net.Name, net.Driver, labels)
		if err != nil && !alreadyExists(err) {
			return fmt.Errorf("service: stack network %s: %w", net.Name, err)
		}
		if err == nil {
			netIDs[net.Name] = n.ID
		}
	}
	volNames := map[string]string{}
	for _, vol := range parsed.Volumes {
		full := project + "_" + vol.Name
		_, err := s.docker.CreateVolume(ctx, full, vol.Driver, labels)
		if err != nil && !alreadyExists(err) {
			return fmt.Errorf("service: stack volume %s: %w", vol.Name, err)
		}
		volNames[vol.Name] = full
	}
	for _, svc := range parsed.Services {
		auth := ""
		if s.registryAuth != nil {
			auth = s.registryAuth(svc.Image)
		}
		pull, err := s.docker.PullImage(ctx, svc.Image, auth)
		if err != nil {
			return fmt.Errorf("service: pull %s: %w", svc.Image, err)
		}
		_, _ = io.Copy(io.Discard, pull)
		_ = pull.Close()

		binds := append([]string{}, svc.Binds...)
		for _, vm := range svc.VolumeMounts {
			src, dest, mode := splitVol(vm)
			full := volNames[src]
			if full == "" {
				full = project + "_" + src
			}
			line := full + ":" + dest
			if mode != "" {
				line += ":" + mode
			}
			binds = append(binds, line)
		}
		svcLabels := map[string]string{}
		for k, v := range svc.Labels {
			svcLabels[k] = v
		}
		svcLabels[domain.ComposeProjectKey] = project
		svcLabels[domain.ComposeServiceKey] = svc.Name
		svcLabels[domain.StackLabelKey] = st.ID
		network := ""
		if len(svc.Networks) > 0 {
			network = project + "_" + svc.Networks[0]
		} else if len(parsed.Networks) == 0 {
			network = project + "_default"
		}
		_, _, err = s.docker.CreateContainer(ctx, domain.ContainerSpec{
			Image:         svc.Image,
			Name:          project + "-" + svc.Name + "-1",
			Env:           svc.Env,
			Ports:         svc.Ports,
			Binds:         binds,
			RestartPolicy: restartOr(svc.Restart),
			Cmd:           svc.Cmd,
			Labels:        svcLabels,
			Network:       network,
			MemoryBytes:   svc.Memory,
		})
		if err != nil && !alreadyExists(err) {
			return fmt.Errorf("service: create %s: %w", svc.Name, err)
		}
		for i, extra := range svc.Networks {
			if i == 0 {
				continue
			}
			_ = s.docker.ConnectNetwork(ctx, project+"_"+extra, project+"-"+svc.Name+"-1")
		}
	}
	return nil
}

func (s *StackService) Down(ctx context.Context, id string, volumes bool) error {
	st, err := s.store.Get(id)
	if err != nil {
		return err
	}
	project := domain.ProjectNameFor(st.Name)
	list, err := s.docker.ListContainers(ctx)
	if err != nil {
		return err
	}
	for _, c := range list {
		if c.Labels[domain.StackLabelKey] != st.ID && c.Labels[domain.ComposeProjectKey] != project {
			continue
		}
		_ = s.docker.StopContainer(ctx, c.ID)
		if err := s.docker.RemoveContainer(ctx, c.ID, true); err != nil {
			return err
		}
	}
	parsed, err := parseComposeYAMLAt(st.YAML, composeDir(s.store.Dir(id), st.ComposeFile), s.store.Dir(id))
	if err != nil {
		return err
	}
	nets := parsed.Networks
	if len(nets) == 0 {
		nets = []parsedNetwork{{Name: "default"}}
	}
	for _, net := range nets {
		_ = s.docker.RemoveNetwork(ctx, project+"_"+net.Name)
	}
	if volumes {
		for _, vol := range parsed.Volumes {
			_ = s.docker.RemoveVolume(ctx, project+"_"+vol.Name, true)
		}
	}
	return nil
}

func restartOr(s string) string {
	if s == "" {
		return "no"
	}
	return s
}

func splitVol(vm string) (src, dest, mode string) {
	parts := strings.Split(vm, ":")
	src, dest = parts[0], parts[1]
	if len(parts) >= 3 {
		mode = parts[2]
	}
	return
}

func alreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already") || strings.Contains(msg, "conflict")
}
