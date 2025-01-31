package util

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/bacalhau-project/bacalhau/pkg/config/types"
	baccrypto "github.com/bacalhau-project/bacalhau/pkg/lib/crypto"
	"github.com/bacalhau-project/bacalhau/pkg/models"
	clientv2 "github.com/bacalhau-project/bacalhau/pkg/publicapi/client/v2"
	"github.com/bacalhau-project/bacalhau/pkg/repo"
	"github.com/bacalhau-project/bacalhau/pkg/version"
)

type Versions struct {
	ClientVersion *models.BuildVersionInfo `json:"clientVersion,omitempty"`
	ServerVersion *models.BuildVersionInfo `json:"serverVersion,omitempty"`
	LatestVersion *models.BuildVersionInfo `json:"latestVersion,omitempty"`
	UpdateMessage string                   `json:"updateMessage,omitempty"`
}

func GetAllVersions(ctx context.Context, cfg types.Bacalhau, api clientv2.API, r *repo.FsRepo) (Versions, error) {
	var err error
	versions := Versions{ClientVersion: version.Get()}

	resp, err := api.Agent().Version(ctx)
	if err != nil {
		return versions, errors.Wrap(err, "error running version command")
	}
	versions.ServerVersion = &models.BuildVersionInfo{
		Major:      resp.Major,
		Minor:      resp.Minor,
		GitVersion: resp.GitVersion,
		GitCommit:  resp.GitCommit,
		BuildDate:  resp.BuildDate,
		GOOS:       resp.GOOS,
		GOARCH:     resp.GOARCH,
	}

	userKeyPath, err := cfg.UserKeyPath()
	if err != nil {
		return versions, err
	}
	userKey, err := baccrypto.LoadUserKey(userKeyPath)
	if err != nil {
		return versions, fmt.Errorf("loading user key: %w", err)
	}

	installationID, err := r.ReadInstallationID()
	if err != nil {
		return versions, fmt.Errorf("reading installationID: %w", err)
	}

	if installationID == "" {
		return versions, errors.Wrap(err, "Installation ID not set")
	}

	updateCheck, err := version.CheckForUpdate(
		ctx,
		versions.ClientVersion,
		versions.ServerVersion,
		userKey.ClientID(),
		installationID,
	)
	if err != nil {
		return versions, errors.Wrap(err, "failed to get latest version")
	} else {
		versions.UpdateMessage = updateCheck.Message
		versions.LatestVersion = updateCheck.Version
	}

	return versions, nil
}

func EnsureValidVersion(ctx context.Context, clientVersion, serverVersion *models.BuildVersionInfo) error {
	if clientVersion == nil {
		log.Ctx(ctx).Warn().Msg("Unable to parse nil client version, skipping version check")
		return nil
	}
	if clientVersion.GitVersion == version.DevelopmentGitVersion {
		log.Ctx(ctx).Debug().Msg("Development client version, skipping version check")
		return nil
	}
	if serverVersion == nil {
		log.Ctx(ctx).Warn().Msg("Unable to parse nil server version, skipping version check")
		return nil
	}
	if serverVersion.GitVersion == version.DevelopmentGitVersion {
		log.Ctx(ctx).Debug().Msg("Development server version, skipping version check")
		return nil
	}
	c, err := semver.NewVersion(clientVersion.GitVersion)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("Unable to parse client version, skipping version check")
		return nil
	}
	s, err := semver.NewVersion(serverVersion.GitVersion)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msg("Unable to parse server version, skipping version check")
		return nil
	}
	if s.GreaterThan(c) {
		return fmt.Errorf(`the server version %s is newer than client version %s, please upgrade your client with the following command:
curl -sL https://get.bacalhau.org/install.sh | bash`,
			serverVersion.GitVersion,
			clientVersion.GitVersion,
		)
	}
	if c.GreaterThan(s) {
		return fmt.Errorf(
			"client version %s is newer than server version %s, please ask your network administrator to update Bacalhau",
			clientVersion.GitVersion,
			serverVersion.GitVersion,
		)
	}
	return nil
}
