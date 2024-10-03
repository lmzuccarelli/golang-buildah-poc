package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/imagebuildah"

	//"github.com/containers/common/libimage"
	"github.com/containers/common/libimage/manifests"
	"github.com/containers/common/pkg/config"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
)

func main() {

	if buildah.InitReexec() {
		return
	}
	unshare.MaybeReexecUsingUserNamespace(false)

	conf, err := config.Default()
	if err != nil {
		panic(err)
	}

	capabilitiesForRoot, err := conf.Capabilities("root", nil, nil)
	if err != nil {
		panic(err)
	}

	platforms := []struct{ OS, Arch, Variant string }{
		{"linux", "amd64", ""},
		{"linux", "arm64", ""},
		{"linux", "ppc64le", ""},
		{"linux", "s390x", ""},
	}

	var jobs *int
	jobs = new(int)
	*jobs = 4

	var catalog = "registry.redhat.io/redhat/redhat-operator-index:v4.15"

	options := define.BuildOptions{
		AddCapabilities:         capabilitiesForRoot,
		AdditionalBuildContexts: nil,
		AdditionalTags:          nil,
		AllPlatforms:            false,
		Annotations:             nil,
		Architecture:            "",
		Args:                    nil,
		BlobDirectory:           "",
		BuildOutput:             "",
		CacheFrom:               nil,
		CacheTo:                 nil,
		CacheTTL:                0,
		CDIConfigDir:            "",
		CNIConfigDir:            "",
		CNIPluginPath:           "",
		CompatVolumes:           types.NewOptionalBool(false),
		ConfidentialWorkload:    define.ConfidentialWorkloadOptions{},
		CPPFlags:                nil,
		CommonBuildOpts:         nil,
		Compression:             imagebuildah.Uncompressed,
		ConfigureNetwork:        buildah.NetworkDisabled,
		ContextDirectory:        "",
		Devices:                 []string{},
		DropCapabilities:        nil,
		Err:                     io.Discard,
		Excludes:                nil,
		ForceRmIntermediateCtrs: false,
		From:                    "",
		GroupAdd:                nil,
		IDMappingOptions:        nil,
		IIDFile:                 "",
		IgnoreFile:              "",
		In:                      nil,
		Isolation:               buildah.IsolationOCIRootless,
		Jobs:                    jobs,
		Labels:                  []string{},
		LayerLabels:             []string{},
		Layers:                  false,
		LogFile:                 "kaka",
		LogRusage:               false,
		LogSplitByPlatform:      false,
		Manifest:                "localhost:5000/redhat/redhat-operator-index:v4.15",
		MaxPullPushRetries:      2,
		NamespaceOptions:        nil,
		NoCache:                 true,
		OS:                      "linux",
		OSFeatures:              nil,
		OSVersion:               "",
		OciDecryptConfig:        nil,
		Out:                     io.Discard,
		Output:                  "",
		OutputFormat:            "application/vnd.oci.image.manifest.v1+json",
		Platforms:               platforms,
		PullPolicy:              define.PullAlways,
		Quiet:                   true,
		RemoveIntermediateCtrs:  false,
		ReportWriter:            io.Discard,
		Runtime:                 "crun",
		RuntimeArgs:             nil,
		RusageLogFile:           "",
		SBOMScanOptions:         nil,
		SignBy:                  "",
		SignaturePolicyPath:     "",
		SkipUnusedStages:        types.NewOptionalBool(false),
		Squash:                  false,
		SystemContext:           nil,
		Target:                  "",
		Timestamp:               nil,
		TransientMounts:         nil,
		UnsetEnvs:               nil,
		UnsetLabels:             nil,
	}

	containerTemplate := `
FROM {{ .Catalog }} AS builder
USER root
RUN rm -fr /configs
COPY ./configs /configs
USER 1001
RUN rm -fr /tmp/cache/*
RUN /bin/opm serve /configs --cache-only --cache-dir=/tmp/cache

FROM {{ .Catalog }} 
USER root
RUN rm -fr /configs
COPY ./configs /configs
USER 1001
RUN rm -fr /tmp/cache/*
COPY --from=builder /tmp/cache /tmp/cache
`

	contents := bytes.NewBufferString("")
	tmpl, err := template.New("Containerfile").Parse(containerTemplate)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(contents, map[string]interface{}{
		"Catalog": catalog,
	})
	if err != nil {
		panic(err)
	}

	// write the Containerfile content to a file
	containerfilePath := filepath.Join("tmp", "Containerfile")
	os.MkdirAll("tmp", 0755)
	defer os.RemoveAll("tmp")

	err = os.WriteFile(containerfilePath, contents.Bytes(), 0755)
	if err != nil {
		panic(err)
	}

	options.DefaultMountsFilePath = ""

	buildStoreOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		panic(err)
	}

	buildStore, err := storage.GetStore(buildStoreOptions)
	if err != nil {
		panic(err)
	}
	defer buildStore.Shutdown(false)

	id, ref, err := imagebuildah.BuildDockerfiles(context.TODO(), buildStore, options, []string{containerfilePath}...)
	if err == nil && options.Manifest != "" {
		fmt.Println(fmt.Sprintf("manifest list id = %s, ref = %s", id, ref.String()))
	}
	if err != nil {
		panic(err)
	}

	var retries *uint
	retries = new(uint)
	*retries = 3

	manifestPushOptions := manifests.PushOptions{
		Store:                  buildStore,
		SystemContext:          newSystemContext(),
		ImageListSelection:     cp.CopyAllImages,
		Instances:              nil,
		RemoveSignatures:       true,
		SignBy:                 "",
		ManifestType:           "application/vnd.oci.image.manifest.v1+json",
		AddCompression:         []string{},
		ForceCompressionFormat: false,
		MaxRetries:             retries,
	}

	dest, err := alltransports.ParseImageName("docker://" + options.Manifest)
	if err != nil {
		panic(err)
	}

	_, list, err := manifests.LoadFromImage(buildStore, id)
	if err != nil {
		panic(err)
	}

	_, digest, err := list.Push(context.TODO(), dest, manifestPushOptions)
	if err != nil {
		panic(err)
	}

	_, err = buildStore.DeleteImage(id, true)
	if err != nil {
		panic(err)
	}

	fmt.Println("digest result ", digest)
}

func newSystemContext() *types.SystemContext {
	ctx := &types.SystemContext{
		RegistriesDirPath:           "",
		ArchitectureChoice:          "",
		OSChoice:                    "",
		VariantChoice:               "",
		BigFilesTemporaryDir:        "",
		DockerInsecureSkipTLSVerify: types.NewOptionalBool(true),
	}
	return ctx
}
