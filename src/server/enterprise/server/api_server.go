package server

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/gogo/protobuf/types"
	"golang.org/x/net/context"

	"github.com/pachyderm/pachyderm/src/client"
	ec "github.com/pachyderm/pachyderm/src/client/enterprise"
	lc "github.com/pachyderm/pachyderm/src/client/license"
	"github.com/pachyderm/pachyderm/src/client/pkg/errors"
	"github.com/pachyderm/pachyderm/src/server/pkg/backoff"
	col "github.com/pachyderm/pachyderm/src/server/pkg/collection"
	"github.com/pachyderm/pachyderm/src/server/pkg/keycache"
	"github.com/pachyderm/pachyderm/src/server/pkg/license"
	"github.com/pachyderm/pachyderm/src/server/pkg/log"
	"github.com/pachyderm/pachyderm/src/server/pkg/serviceenv"
)

const (
	// enterpriseTokenKey is the constant key we use that maps to an Enterprise
	// token that a user has given us. This is what we check to know if a
	// Pachyderm cluster supports enterprise features
	enterpriseTokenKey = "token"
)

type apiServer struct {
	pachLogger log.Logger
	env        *serviceenv.ServiceEnv

	enterpriseTokenCache *keycache.Cache

	// enterpriseToken is a collection containing at most one Pachyderm enterprise
	// token
	enterpriseToken col.Collection
}

func (a *apiServer) LogReq(request interface{}) {
	a.pachLogger.Log(request, nil, nil, 0)
}

// NewEnterpriseServer returns an implementation of ec.APIServer.
func NewEnterpriseServer(env *serviceenv.ServiceEnv, etcdPrefix string) (ec.APIServer, error) {
	defaultExpires, err := types.TimestampProto(time.Time{})
	if err != nil {
		return nil, err
	}
	defaultEnterpriseRecord := &ec.EnterpriseRecord{
		License: &ec.LicenseRecord{Expires: defaultExpires},
	}
	enterpriseToken := col.NewCollection(
		env.GetEtcdClient(),
		etcdPrefix,
		nil,
		&ec.EnterpriseRecord{},
		nil,
		nil,
	)

	s := &apiServer{
		pachLogger:           log.NewLogger("enterprise.API"),
		env:                  env,
		enterpriseTokenCache: keycache.NewCache(enterpriseToken, enterpriseTokenKey, defaultEnterpriseRecord),
		enterpriseToken:      enterpriseToken,
	}
	go s.enterpriseTokenCache.Watch()
	return s, nil
}

func (a *apiServer) heartbeat(ctx context.Context, licenseServer, id, secret string) error {
	localClient := a.env.GetPachClient(ctx)
	versionResp, err := localClient.Version()
	if err != nil {
		return err
	}

	authEnabled, err := localClient.IsAuthActive()
	if err != nil {
		return err
	}

	pachClient, err := client.NewFromAddress(licenseServer)
	if err != nil {
		return err
	}

	resp, err := pachClient.License.Heartbeat(ctx, &lc.HeartbeatRequest{
		Id:          id,
		Secret:      secret,
		Version:     versionResp,
		AuthEnabled: authEnabled,
	})

	if err != nil {
		return err
	}

	if _, err := col.NewSTM(ctx, a.env.GetEtcdClient(), func(stm col.STM) error {
		e := a.enterpriseToken.ReadWrite(stm)
		return e.Put(enterpriseTokenKey, &ec.EnterpriseRecord{
			LicenseServer: licenseServer,
			Id:            id,
			Secret:        secret,
			LastHeartbeat: types.TimestampNow(),
			License:       resp.License,
		})
	}); err != nil {
		return err
	}
	return nil
}

// Activate implements the Activate RPC
func (a *apiServer) Activate(ctx context.Context, req *ec.ActivateRequest) (resp *ec.ActivateResponse, retErr error) {
	a.LogReq(req)
	defer func(start time.Time) { a.pachLogger.Log(req, resp, retErr, time.Since(start)) }(time.Now())

	if err := a.heartbeat(ctx, req.LicenseServer, req.Id, req.Secret); err != nil {
		return nil, err
	}

	// Wait until watcher observes the write
	if err := backoff.Retry(func() error {
		record, ok := a.enterpriseTokenCache.Load().(*ec.EnterpriseRecord)
		if !ok {
			return errors.Errorf("could not retrieve enterprise expiration time")
		}
		expiration, err := types.TimestampFromProto(record.License.Expires)
		if err != nil {
			return errors.Wrapf(err, "could not parse expiration timestamp")
		}
		if expiration.IsZero() {
			return errors.Errorf("enterprise not activated")
		}
		return nil
	}, backoff.RetryEvery(time.Second)); err != nil {
		return nil, err
	}

	return &ec.ActivateResponse{}, nil
}

// GetState returns the current state of the cluster's Pachyderm Enterprise key (ACTIVE, EXPIRED, or NONE), without the activation code
func (a *apiServer) GetState(ctx context.Context, req *ec.GetStateRequest) (resp *ec.GetStateResponse, retErr error) {
	record, err := a.getEnterpriseRecord()
	if err != nil {
		return nil, err
	}

	resp = &ec.GetStateResponse{
		Info:  record.Info,
		State: record.State,
	}

	if record.ActivationCode != "" {
		activationCode, err := license.Unmarshal(record.ActivationCode)
		if err != nil {
			return nil, err
		}

		activationCode.Signature = ""
		activationCodeStr, err := json.Marshal(activationCode)
		if err != nil {
			return nil, err
		}

		resp.ActivationCode = base64.StdEncoding.EncodeToString(activationCodeStr)
	}

	return resp, nil
}

// GetActivationCode returns the current state of the cluster's Pachyderm Enterprise key (ACTIVE, EXPIRED, or NONE), including the enterprise activation code
func (a *apiServer) GetActivationCode(ctx context.Context, req *ec.GetActivationCodeRequest) (resp *ec.GetActivationCodeResponse, retErr error) {
	a.LogReq(req)
	defer func(start time.Time) { a.pachLogger.Log(req, resp, retErr, time.Since(start)) }(time.Now())
	return a.getEnterpriseRecord()
}

func (a *apiServer) getEnterpriseRecord() (*ec.GetActivationCodeResponse, error) {
	record, ok := a.enterpriseTokenCache.Load().(*ec.EnterpriseRecord)
	if !ok {
		return nil, errors.Errorf("could not retrieve enterprise expiration time")
	}
	expiration, err := types.TimestampFromProto(record.License.Expires)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse expiration timestamp")
	}
	if expiration.IsZero() {
		return &ec.GetActivationCodeResponse{State: ec.State_NONE}, nil
	}
	resp := &ec.GetActivationCodeResponse{
		Info: &ec.TokenInfo{
			Expires: record.License.Expires,
		},
		ActivationCode: record.License.ActivationCode,
	}
	if time.Now().After(expiration) {
		resp.State = ec.State_EXPIRED
	} else {
		resp.State = ec.State_ACTIVE
	}
	return resp, nil
}

// Deactivate deletes the current cluster's enterprise token, and puts the
// cluster in the "NONE" enterprise state.
func (a *apiServer) Deactivate(ctx context.Context, req *ec.DeactivateRequest) (resp *ec.DeactivateResponse, retErr error) {
	a.LogReq(req)
	defer func(start time.Time) { a.pachLogger.Log(req, resp, retErr, time.Since(start)) }(time.Now())

	if _, err := col.NewSTM(ctx, a.env.GetEtcdClient(), func(stm col.STM) error {
		err := a.enterpriseToken.ReadWrite(stm).Delete(enterpriseTokenKey)
		if err != nil && !col.IsErrNotFound(err) {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Wait until watcher observes the write
	if err := backoff.Retry(func() error {
		record, ok := a.enterpriseTokenCache.Load().(*ec.EnterpriseRecord)
		if !ok {
			return errors.Errorf("could not retrieve enterprise expiration time")
		}
		expiration, err := types.TimestampFromProto(record.License.Expires)
		if err != nil {
			return errors.Wrapf(err, "could not parse expiration timestamp")
		}
		if !expiration.IsZero() {
			return errors.Errorf("enterprise still activated")
		}
		return nil
	}, backoff.RetryEvery(time.Second)); err != nil {
		return nil, err
	}

	return &ec.DeactivateResponse{}, nil
}
