package rigging

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/n3wscott/rigging/pkg/installer"
	"github.com/n3wscott/rigging/pkg/lifecycle"
	yaml "github.com/n3wscott/rigging/pkg/manifest"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	kntest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

type Rigging interface {
	Install(dirs []string, config map[string]string) error
	Uninstall() error
	Objects() []v1.ObjectReference
	WaitForReadyOrDone(ref v1.ObjectReference, timeout time.Duration) (string, error)
	LogsFor(ref v1.ObjectReference) (string, error)
	Namespace() string
}

// RegisterPackage registers an interest in producing an image based on the
// provide package.
var RegisterPackage = installer.RegisterPackage

type Option func(Rigging) error

func New(opts ...Option) (Rigging, error) {
	r := &riggingImpl{
		configDirTemplate: DefaultConfigDirTemplate,
		runInParallel:     true,
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}

	// Set other defaults if not set.
	if r.rootDir == "" {
		_, filename, _, _ := runtime.Caller(2)
		r.rootDir = filepath.Dir(filename)
	}
	if r.name == "" {
		r.name = "rigging"
	}

	return r, nil
}

func NewInstall(opts []Option, dirs []string, config map[string]string) (Rigging, error) {
	rig, err := New(opts...)
	if err != nil {
		return nil, err
	}

	if err := rig.Install(dirs, config); err != nil {
		return rig, err
	}

	return rig, err
}

const (
	DefaultConfigDirTemplate = "%s/config/%s"
)

// WithConfigDirTemplate lets you change the configuration directory template.
// This value will be used to produce the fill file path of manifest files. And
// used like sprintf(dir, rootDir, name).
// By default:
//  - dir this value will be "%s/config/%s".
//  - rootDir is the directory of the caller to New.
//  - name is provided in a list for install
func WithConfigDirTemplate(dir string) Option {
	return func(r Rigging) error {
		if ri, ok := r.(*riggingImpl); ok {
			if !strings.HasPrefix(dir, "%s") || !strings.HasSuffix(dir, "%s") {
				return errors.New("invalid format; WithConfigDirTemplate(dir), expecting dir like '%s/<custom>/%s'")
			}

			ri.configDirTemplate = dir
			return nil
		}
		return errors.New("unknown rigging implementation")
	}
}

func WithRootDir(dir string) Option {
	return func(r Rigging) error {
		if ri, ok := r.(*riggingImpl); ok {
			ri.rootDir = dir
			return nil
		}
		return errors.New("unknown rigging implementation")
	}
}

func WithImages(images map[string]string) Option {
	return func(r Rigging) error {
		if ri, ok := r.(*riggingImpl); ok {
			ri.images = images
			return nil
		}
		return errors.New("unknown rigging implementation")
	}
}

type riggingImpl struct {
	configDirTemplate string
	rootDir           string
	runInParallel     bool
	namespace         string
	name              string
	images            map[string]string

	client   *lifecycle.Client
	manifest yaml.Manifest
}

// Install implements Rigging.Install
// Install will do the following:
//  1. Create testing client and testing namespace.
//  2. Produce all images registered.
//  3. Pass the yaml config through the templating system.
//  4. Apply the yaml config to the cluster.
//
func (r *riggingImpl) Install(dirs []string, config map[string]string) error {

	// 1. Create testing client and testing namespace.

	if err := r.createEnvironment(); err != nil {
		return err
	}

	// 2. Produce all images registered.

	cfg, err := r.updateConfig(config)
	if err != nil {
		return err
	}

	// 3. Pass the yaml config through the templating system.

	yamls := r.yamlDirs(dirs)

	for i, p := range yamls {
		yamls[i] = installer.ParseTemplates(p, cfg)
	}
	path := strings.Join(yamls, ",")

	manifest, err := yaml.NewYamlManifest(path, true, r.client.Dynamic)
	if err != nil {
		return err
	}
	r.manifest = manifest

	// 4. Apply yaml.
	if err := r.manifest.ApplyAll(); err != nil {
		return err
	}

	// Temp
	refs := r.manifest.References()
	log.Println("Created:")
	for _, ref := range refs {
		log.Println(ref)
	}

	return nil
}

func (r *riggingImpl) createEnvironment() error {
	if r.namespace == "" {
		baseName := helpers.AppendRandomString(r.name)
		r.namespace = helpers.MakeK8sNamePrefix(baseName)
	}

	client, err := lifecycle.NewClient(
		kntest.Flags.Kubeconfig,
		kntest.Flags.Cluster,
		r.namespace)
	if err != nil {
		return fmt.Errorf("could not initialize clients: %v", err)
	}
	r.client = client

	if err := r.client.CreateNamespaceIfNeeded(); err != nil {
		return err
	}
	return nil
}

func (r *riggingImpl) updateConfig(config map[string]string) (map[string]interface{}, error) {
	if r.images == nil {
		ic, err := installer.ProduceImages()
		if err != nil {
			return nil, err
		}
		r.images = ic
	}

	cfg := make(map[string]interface{})
	for k, v := range config {
		cfg[k] = v
	}
	// Implement template contract for Rigging:
	cfg["images"] = r.images
	cfg["namespace"] = r.Namespace()
	return cfg, nil
}

func (r *riggingImpl) yamlDirs(dirs []string) []string {
	yamls := make([]string, 0, len(dirs))
	for _, d := range dirs {
		yamls = append(yamls, fmt.Sprintf(r.configDirTemplate, r.rootDir, d))
	}

	return yamls
}

// Uninstall implements Rigging.Uninstall
func (r *riggingImpl) Uninstall() error {
	// X. Delete yaml.
	if err := r.manifest.DeleteAll(); err != nil {
		return err
	}

	// Just chill for tick.
	time.Sleep(5 * time.Second)

	// TODO: wait for resources to be finished deleting...

	// Y. Delete namespace.
	if err := r.client.DeleteNamespaceIfNeeded(); err != nil {
		return err
	}
	return nil
}

// Objects implements Rigging.Objects
func (r *riggingImpl) Objects() []v1.ObjectReference {
	return r.manifest.References()
}

// Namespace implements Rigging.Namespace
func (r *riggingImpl) Namespace() string {
	return r.namespace
}

func (r *riggingImpl) WaitForReadyOrDone(ref v1.ObjectReference, timeout time.Duration) (string, error) {
	k := ref.GroupVersionKind()
	gvk, _ := meta.UnsafeGuessKindToResource(k)

	switch gvk.Resource {
	case "jobs":
		out, err := r.client.WaitUntilJobDone(ref.Namespace, ref.Name, timeout)
		if err != nil {
			return "", err
		}
		return out, nil

	default:
		err := r.client.WaitForResourceReady(ref.Namespace, ref.Name, gvk, timeout)
		if err != nil {
			return "", err
		}
	}

	return "", nil
}

func (r *riggingImpl) LogsFor(ref v1.ObjectReference) (string, error) {
	k := ref.GroupVersionKind()
	gvk, _ := meta.UnsafeGuessKindToResource(k)

	return r.client.LogsFor(ref.Namespace, ref.Name, gvk)
}
