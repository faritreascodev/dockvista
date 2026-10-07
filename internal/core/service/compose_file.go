package service

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"dockvista/internal/core/domain"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Networks map[string]composeNetwork `yaml:"networks"`
	Volumes  map[string]composeVolume  `yaml:"volumes"`
}

type composeService struct {
	Image       string              `yaml:"image"`
	Build       any                 `yaml:"build"`
	Command     any                 `yaml:"command"`
	Environment any                 `yaml:"environment"`
	Ports       []any               `yaml:"ports"`
	Volumes     []any               `yaml:"volumes"`
	Restart     string              `yaml:"restart"`
	DependsOn   any                 `yaml:"depends_on"`
	Labels      any                 `yaml:"labels"`
	Networks    any                 `yaml:"networks"`
	MemLimit    any                 `yaml:"mem_limit"`
}

type composeNetwork struct {
	Driver string `yaml:"driver"`
}

type composeVolume struct {
	Driver string `yaml:"driver"`
}

type parsedStack struct {
	Services []parsedService
	Networks []parsedNetwork
	Volumes  []parsedNamedVolume
}

type parsedService struct {
	Name      string
	Image     string
	Cmd       []string
	Env       []string
	Ports     []domain.PortBinding
	Binds     []string
	VolumeMounts []string // named volume: "vol:containerPath"
	Restart   string
	DependsOn []string
	Labels    map[string]string
	Networks  []string
	Memory    int64
}

type parsedNetwork struct {
	Name   string
	Driver string
}

type parsedNamedVolume struct {
	Name   string
	Driver string
}

func parseComposeYAML(yamlBody, workDir string) (parsedStack, error) {
	return parseComposeYAMLAt(yamlBody, workDir, workDir)
}

func parseComposeYAMLAt(yamlBody, composeDir, sandboxRoot string) (parsedStack, error) {
	var raw composeFile
	if err := yaml.Unmarshal([]byte(yamlBody), &raw); err != nil {
		return parsedStack{}, fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	if len(raw.Services) == 0 {
		return parsedStack{}, fmt.Errorf("%w: compose file has no services", domain.ErrInvalidInput)
	}
	out := parsedStack{}
	for name, net := range raw.Networks {
		driver := net.Driver
		if driver == "" {
			driver = "bridge"
		}
		out.Networks = append(out.Networks, parsedNetwork{Name: name, Driver: driver})
	}
	for name, vol := range raw.Volumes {
		driver := vol.Driver
		if driver == "" {
			driver = "local"
		}
		out.Volumes = append(out.Volumes, parsedNamedVolume{Name: name, Driver: driver})
	}
	named := map[string]bool{}
	for _, v := range out.Volumes {
		named[v.Name] = true
	}
	for name, svc := range raw.Services {
		if svc.Build != nil && strings.TrimSpace(svc.Image) == "" {
			return parsedStack{}, domain.ErrBuildUnsupported
		}
		if strings.TrimSpace(svc.Image) == "" {
			return parsedStack{}, fmt.Errorf("%w: service %s has no image", domain.ErrInvalidInput, name)
		}
		ps := parsedService{
			Name:    name,
			Image:   strings.TrimSpace(svc.Image),
			Cmd:     asStringSlice(svc.Command),
			Env:     asEnv(svc.Environment),
			Restart: svc.Restart,
			Labels:  asStringMap(svc.Labels),
			DependsOn: asDepends(svc.DependsOn),
			Networks: asNetworkNames(svc.Networks),
			Memory:  asBytes(svc.MemLimit),
		}
		for _, p := range svc.Ports {
			pb, err := parsePort(p)
			if err != nil {
				return parsedStack{}, err
			}
			if pb.ContainerPort != "" {
				ps.Ports = append(ps.Ports, pb)
			}
		}
		for _, v := range svc.Volumes {
			bind, volMount, err := parseVolumeLine(v, composeDir, sandboxRoot, named)
			if err != nil {
				return parsedStack{}, err
			}
			if bind != "" {
				ps.Binds = append(ps.Binds, bind)
			}
			if volMount != "" {
				ps.VolumeMounts = append(ps.VolumeMounts, volMount)
			}
		}
		out.Services = append(out.Services, ps)
	}
	ordered, err := topoServices(out.Services)
	if err != nil {
		return parsedStack{}, err
	}
	out.Services = ordered
	return out, nil
}

func parseVolumeLine(raw any, composeDir, sandboxRoot string, named map[string]bool) (bind, volMount string, err error) {
	s, ok := raw.(string)
	if !ok {
		return "", "", fmt.Errorf("%w: long volume syntax is not supported", domain.ErrInvalidInput)
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("%w: volume %q", domain.ErrInvalidInput, s)
	}
	src, dest := parts[0], parts[1]
	mode := ""
	if len(parts) >= 3 {
		mode = parts[2]
	}
	if named[src] || (!strings.Contains(src, "/") && !strings.HasPrefix(src, ".")) {
		if mode != "" {
			return "", src + ":" + dest + ":" + mode, nil
		}
		return "", src + ":" + dest, nil
	}
	if filepath.IsAbs(src) || strings.HasPrefix(src, "/") || strings.HasPrefix(src, "\\") {
		return "", "", domain.ErrBindOutsideStack
	}
	abs, err := filepath.Abs(filepath.Join(composeDir, src))
	if err != nil {
		return "", "", domain.ErrInvalidInput
	}
	rel, err := filepath.Rel(sandboxRoot, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", domain.ErrBindOutsideStack
	}
	if mode != "" {
		return abs + ":" + dest + ":" + mode, "", nil
	}
	return abs + ":" + dest, "", nil
}

func parsePort(raw any) (domain.PortBinding, error) {
	s, ok := raw.(string)
	if !ok {
		n, ok := asInt(raw)
		if !ok {
			return domain.PortBinding{}, fmt.Errorf("%w: port", domain.ErrInvalidInput)
		}
		return domain.PortBinding{ContainerPort: strconv.Itoa(n), Protocol: "tcp"}, nil
	}
	proto := "tcp"
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		proto = s[i+1:]
		s = s[:i]
	}
	host, cont := "", s
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		host, cont = s[:i], s[i+1:]
		if j := strings.LastIndexByte(host, ':'); j >= 0 {
			host = host[j+1:]
		}
	}
	if host == "" {
		return domain.PortBinding{ContainerPort: cont, Protocol: proto}, nil
	}
	return domain.PortBinding{HostPort: host, ContainerPort: cont, Protocol: proto}, nil
}

func asStringSlice(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if t == "" {
			return nil
		}
		return []string{"/bin/sh", "-c", t}
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	default:
		return nil
	}
}

func asEnv(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(t))
		for k, val := range t {
			out = append(out, k+"="+fmt.Sprint(val))
		}
		return out
	default:
		return nil
	}
}

func asStringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	out := map[string]string{}
	for k, val := range m {
		out[k] = fmt.Sprint(val)
	}
	return out
}

func asDepends(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(t))
		for k := range t {
			out = append(out, k)
		}
		return out
	default:
		return nil
	}
}

func asNetworkNames(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(t))
		for k := range t {
			out = append(out, k)
		}
		return out
	default:
		return nil
	}
}

func asBytes(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	case float64:
		return int64(t)
	case string:
		n, err := strconv.ParseInt(strings.TrimRight(strings.ToLower(t), "bmgk"), 10, 64)
		if err != nil {
			return 0
		}
		s := strings.ToLower(t)
		switch {
		case strings.HasSuffix(s, "g"):
			return n << 30
		case strings.HasSuffix(s, "m"):
			return n << 20
		case strings.HasSuffix(s, "k"):
			return n << 10
		default:
			return n
		}
	default:
		return 0
	}
}

func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	default:
		return 0, false
	}
}

func topoServices(in []parsedService) ([]parsedService, error) {
	byName := map[string]parsedService{}
	for _, s := range in {
		byName[s.Name] = s
	}
	seen := map[string]int{}
	var out []parsedService
	var visit func(string) error
	visit = func(name string) error {
		st := seen[name]
		if st == 1 {
			return fmt.Errorf("%w: cyclic depends_on", domain.ErrInvalidInput)
		}
		if st == 2 {
			return nil
		}
		svc, ok := byName[name]
		if !ok {
			return fmt.Errorf("%w: unknown service %s", domain.ErrInvalidInput, name)
		}
		seen[name] = 1
		for _, dep := range svc.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		seen[name] = 2
		out = append(out, svc)
		return nil
	}
	for _, s := range in {
		if err := visit(s.Name); err != nil {
			return nil, err
		}
	}
	return out, nil
}
