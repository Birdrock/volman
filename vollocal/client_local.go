package vollocal

import (
	"errors"
	"time"

	"github.com/tedsuo/ifrit"

	"os"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/volman"
	"code.cloudfoundry.org/volman/voldiscoverers"
	"github.com/tedsuo/ifrit/grouper"
)

const (
	volmanMountErrorsCounter   = "VolmanMountErrors"
	volmanMountDuration        = "VolmanMountDuration"
	volmanUnmountErrorsCounter = "VolmanUnmountErrors"
	volmanUnmountDuration      = "VolmanUnmountDuration"
)

var (
	pluginMountDurations   = map[string]string{}
	pluginUnmountDurations = map[string]string{}
)

type DriverConfig struct {
	DriverPaths     []string
	CSIPaths        []string
	SyncInterval    time.Duration
	CSIMountRootDir string
	MapfsPath       string
}

func NewDriverConfig() DriverConfig {
	return DriverConfig{
		SyncInterval: time.Second * 30,
	}
}

type localClient struct {
	pluginRegistry volman.PluginRegistry
	metronClient   loggingclient.IngressClient
	clock          clock.Clock
}

func NewServer(logger lager.Logger, metronClient loggingclient.IngressClient, config DriverConfig) (volman.Manager, ifrit.Runner) {
	clock := clock.NewClock()
	registry := NewPluginRegistry()

	dockerDiscoverer := voldiscoverers.NewDockerDriverDiscoverer(logger, registry, config.DriverPaths)
	csiDiscoverer := voldiscoverers.NewCsiPluginDiscoverer(logger, registry, config.CSIPaths, config.CSIMountRootDir, config.MapfsPath)

	syncer := NewSyncer(logger, registry, []volman.Discoverer{dockerDiscoverer, csiDiscoverer}, config.SyncInterval, clock)
	purger := NewMountPurger(logger, registry)

	grouper := grouper.NewOrdered(os.Kill, grouper.Members{grouper.Member{Name: "volman-syncer", Runner: syncer.Runner()}, grouper.Member{Name: "volman-purger", Runner: purger.Runner()}})

	return NewLocalClient(logger, registry, metronClient, clock), grouper
}

func NewLocalClient(logger lager.Logger, registry volman.PluginRegistry, metronClient loggingclient.IngressClient, clock clock.Clock) volman.Manager {
	return &localClient{
		pluginRegistry: registry,
		metronClient:   metronClient,
		clock:          clock,
	}
}

func (client *localClient) ListDrivers(logger lager.Logger) (volman.ListDriversResponse, error) {
	logger = logger.Session("list-drivers")
	logger.Info("start")
	defer logger.Info("end")

	var infoResponses []volman.InfoResponse
	plugins := client.pluginRegistry.Plugins()

	for name, _ := range plugins {
		infoResponses = append(infoResponses, volman.InfoResponse{Name: name})
	}

	logger.Debug("listing-drivers", lager.Data{"drivers": infoResponses})
	return volman.ListDriversResponse{Drivers: infoResponses}, nil
}

func (client *localClient) Mount(logger lager.Logger, pluginId string, volumeId string, config map[string]interface{}) (volman.MountResponse, error) {
	logger = logger.Session("mount")
	logger.Info("start")
	defer logger.Info("end")

	mountStart := client.clock.Now()

	defer func() {
		sendMountDurationMetrics(logger, client.metronClient, time.Since(mountStart), pluginId)
	}()

	logger.Debug("plugin-mounting-volume", lager.Data{"pluginId": pluginId, "volumeId": volumeId})

	plugin, found := client.pluginRegistry.Plugin(pluginId)
	if !found {
		err := errors.New("Plugin '" + pluginId + "' not found in list of known plugins")
		logger.Error("mount-plugin-lookup-error", err)
		client.metronClient.IncrementCounter(volmanMountErrorsCounter)
		return volman.MountResponse{}, err
	}

	mountResponse, err := plugin.Mount(logger, volumeId, config)

	if err != nil {
		client.metronClient.IncrementCounter(volmanMountErrorsCounter)
		if voldriverSafeErr, ok := err.(voldriver.SafeError); ok {
			return volman.MountResponse{}, volman.SafeError{SafeDescription: voldriverSafeErr.SafeDescription}
		}
		return volman.MountResponse{}, err
	}

	return mountResponse, nil
}

func sendMountDurationMetrics(logger lager.Logger, metronClient loggingclient.IngressClient, duration time.Duration, pluginId string) {
	err := metronClient.SendDuration(volmanMountDuration, duration)
	if err != nil {
		logger.Error("failed-to-send-volman-mount-duration-metric", err)
	}

	m, ok := pluginMountDurations[pluginId]
	if !ok {
		m = "VolmanMountDurationFor" + pluginId
		pluginMountDurations[pluginId] = m
	}
	err = metronClient.SendDuration(m, duration)
	if err != nil {
		logger.Error("failed-to-send-volman-mount-duration-metric", err)
	}
}

func sendUnmountDurationMetrics(logger lager.Logger, metronClient loggingclient.IngressClient, duration time.Duration, pluginId string) {
	err := metronClient.SendDuration(volmanUnmountDuration, duration)
	if err != nil {
		logger.Error("failed-to-send-volman-unmount-duration-metric", err)
	}

	m, ok := pluginUnmountDurations[pluginId]
	if !ok {
		m = "VolmanUnmountDurationFor" + pluginId
		pluginUnmountDurations[pluginId] = m
	}
	err = metronClient.SendDuration(m, duration)
	if err != nil {
		logger.Error("failed-to-send-volman-unmount-duration-metric", err)
	}
}

func (client *localClient) Unmount(logger lager.Logger, pluginId string, volumeId string) error {
	logger = logger.Session("unmount")
	logger.Info("start")
	defer logger.Info("end")
	logger.Debug("unmounting-volume", lager.Data{"volumeName": volumeId})

	unmountStart := client.clock.Now()

	defer func() {
		sendUnmountDurationMetrics(logger, client.metronClient, time.Since(unmountStart), pluginId)
	}()

	plugin, found := client.pluginRegistry.Plugin(pluginId)
	if !found {
		err := errors.New("Plugin '" + pluginId + "' not found in list of known plugins")
		logger.Error("mount-plugin-lookup-error", err)
		client.metronClient.IncrementCounter(volmanUnmountErrorsCounter)
		return err
	}

	err := plugin.Unmount(logger, volumeId)
	if err != nil {
		client.metronClient.IncrementCounter(volmanUnmountErrorsCounter)
		logger.Error("unmount-failed", err)

		if voldriverSafeErr, ok := err.(voldriver.SafeError); ok {
			return volman.SafeError{SafeDescription: voldriverSafeErr.SafeDescription}
		}
		return err
	}

	return nil
}
