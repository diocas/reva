// Copyright 2018-2019 CERN
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// In applying this license, CERN does not waive the privileges and immunities
// granted to it by virtue of its status as an Intergovernmental Organization
// or submit itself to any jurisdiction.

package gatewaysvc

import (
	"context"
	"fmt"

	rpcpb "github.com/cs3org/go-cs3apis/cs3/rpc"
	storageproviderv0alphapb "github.com/cs3org/go-cs3apis/cs3/storageprovider/v0alpha"
	usershareproviderv0alphapb "github.com/cs3org/go-cs3apis/cs3/usershareprovider/v0alpha"
	"github.com/cs3org/reva/cmd/revad/svcs/grpcsvcs/pool"
	"github.com/cs3org/reva/cmd/revad/svcs/grpcsvcs/status"
	"github.com/cs3org/reva/pkg/appctx"
	"github.com/pkg/errors"
)

// TODO(labkode): add multi-phase commit logic when commit share or commit ref is enabled.
func (s *svc) CreateShare(ctx context.Context, req *usershareproviderv0alphapb.CreateShareRequest) (*usershareproviderv0alphapb.CreateShareResponse, error) {
	log := appctx.GetLogger(ctx)

	c, err := pool.GetUserShareProviderClient(s.c.UserShareProviderEndpoint)
	if err != nil {
		log.Err(err).Msg("gatewaysvc: error getting usershareprovider client")
		return &usershareproviderv0alphapb.CreateShareResponse{
			Status: &rpcpb.Status{
				Code: rpcpb.Code_CODE_INTERNAL,
			},
		}, nil
	}

	res, err := c.CreateShare(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "gatewaysvc: error calling CreateShare")
	}

	if res.Status.Code != rpcpb.Code_CODE_OK {
		return res, nil
	}

	// if we don't need to commit we return earlier
	if !s.c.CommitShareToStorageGrant && !s.c.CommitShareToStorageRef {
		return res, nil
	}

	// TODO(labkode): if both commits are enabled they could be done concurrently.
	if s.c.CommitShareToStorageGrant {
		grantReq := &storageproviderv0alphapb.AddGrantRequest{
			Ref: &storageproviderv0alphapb.Reference{
				Spec: &storageproviderv0alphapb.Reference_Id{
					Id: req.ResourceInfo.Id,
				},
			},
			Grant: &storageproviderv0alphapb.Grant{
				Grantee:     req.Grant.Grantee,
				Permissions: req.Grant.Permissions.Permissions,
			},
		}
		grantRes, err := s.AddGrant(ctx, grantReq)
		if err != nil {
			return nil, errors.Wrap(err, "gatewaysvc: error calling AddGrant")
		}
		if grantRes.Status.Code != rpcpb.Code_CODE_OK {
			res := &usershareproviderv0alphapb.CreateShareResponse{
				Status: status.NewInternal(ctx, "error committing share to storage grant"),
			}
			return res, nil
		}
	}

	return res, nil
}

func (s *svc) RemoveShare(ctx context.Context, req *usershareproviderv0alphapb.RemoveShareRequest) (*usershareproviderv0alphapb.RemoveShareResponse, error) {
	log := appctx.GetLogger(ctx)

	c, err := pool.GetUserShareProviderClient(s.c.UserShareProviderEndpoint)
	if err != nil {
		log.Err(err).Msg("gatewaysvc: error getting usershareprovider client")
		return &usershareproviderv0alphapb.RemoveShareResponse{
			Status: status.NewInternal(ctx, "error getting user share provider client"),
		}, nil
	}

	// if we need to commit the share, we need the resource it points to.
	var share *usershareproviderv0alphapb.Share
	if s.c.CommitShareToStorageGrant || s.c.CommitShareToStorageRef {
		getShareReq := &usershareproviderv0alphapb.GetShareRequest{
			Ref: req.Ref,
		}
		getShareRes, err := c.GetShare(ctx, getShareReq)
		if err != nil {
			return nil, errors.Wrap(err, "gatewaysvc: error calling GetShare")
		}

		if getShareRes.Status.Code != rpcpb.Code_CODE_OK {
			res := &usershareproviderv0alphapb.RemoveShareResponse{
				Status: status.NewInternal(ctx, "error getting share when committing to the storage"),
			}
			return res, nil
		}
		share = getShareRes.Share
	}

	res, err := c.RemoveShare(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "gatewaysvc: error calling RemoveShare")
	}

	// if we don't need to commit we return earlier
	if !s.c.CommitShareToStorageGrant && !s.c.CommitShareToStorageRef {
		return res, nil
	}

	// TODO(labkode): if both commits are enabled they could be done concurrently.
	if s.c.CommitShareToStorageGrant {
		grantReq := &storageproviderv0alphapb.RemoveGrantRequest{
			Ref: &storageproviderv0alphapb.Reference{
				Spec: &storageproviderv0alphapb.Reference_Id{
					Id: share.ResourceId,
				},
			},
			Grant: &storageproviderv0alphapb.Grant{
				Grantee:     share.Grantee,
				Permissions: share.Permissions.Permissions,
			},
		}

		grantRes, err := s.RemoveGrant(ctx, grantReq)
		if err != nil {
			return nil, errors.Wrap(err, "gatewaysvc: error calling RemoveGrant")
		}
		if grantRes.Status.Code != rpcpb.Code_CODE_OK {
			res := &usershareproviderv0alphapb.RemoveShareResponse{
				Status: status.NewInternal(ctx, "error removing storage grant"),
			}
			return res, nil
		}
	}

	return res, nil
}

// TODO(labkode): we need to validate share state vs storage grant and storage ref
// If there are any inconsitencies, the share needs to be flag as invalid and a background process
// or active fix needs to be performed.
func (s *svc) GetShare(ctx context.Context, req *usershareproviderv0alphapb.GetShareRequest) (*usershareproviderv0alphapb.GetShareResponse, error) {
	return s.getShare(ctx, req)
}

func (s *svc) getShare(ctx context.Context, req *usershareproviderv0alphapb.GetShareRequest) (*usershareproviderv0alphapb.GetShareResponse, error) {
	log := appctx.GetLogger(ctx)

	c, err := pool.GetUserShareProviderClient(s.c.UserShareProviderEndpoint)
	if err != nil {
		log.Err(err).Msg("gatewaysvc: error getting usershareprovider client")
		return &usershareproviderv0alphapb.GetShareResponse{
			Status: &rpcpb.Status{
				Code: rpcpb.Code_CODE_INTERNAL,
			},
		}, nil
	}

	res, err := c.GetShare(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "gatewaysvc: error calling GetShare")
	}

	return res, nil
}

// TODO(labkode): read GetShare comment.
func (s *svc) ListShares(ctx context.Context, req *usershareproviderv0alphapb.ListSharesRequest) (*usershareproviderv0alphapb.ListSharesResponse, error) {
	log := appctx.GetLogger(ctx)

	c, err := pool.GetUserShareProviderClient(s.c.UserShareProviderEndpoint)
	if err != nil {
		log.Err(err).Msg("gatewaysvc: error getting usershareprovider client")
		return &usershareproviderv0alphapb.ListSharesResponse{
			Status: &rpcpb.Status{
				Code: rpcpb.Code_CODE_INTERNAL,
			},
		}, nil
	}

	res, err := c.ListShares(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "gatewaysvc: error calling ListShares")
	}

	return res, nil
}

func (s *svc) UpdateShare(ctx context.Context, req *usershareproviderv0alphapb.UpdateShareRequest) (*usershareproviderv0alphapb.UpdateShareResponse, error) {
	log := appctx.GetLogger(ctx)

	c, err := pool.GetUserShareProviderClient(s.c.UserShareProviderEndpoint)
	if err != nil {
		log.Err(err).Msg("gatewaysvc: error getting usershareprovider client")
		return &usershareproviderv0alphapb.UpdateShareResponse{
			Status: &rpcpb.Status{
				Code: rpcpb.Code_CODE_INTERNAL,
			},
		}, nil
	}

	res, err := c.UpdateShare(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "gatewaysvc: error calling UpdateShare")
	}

	// if we don't need to commit we return earlier
	if !s.c.CommitShareToStorageGrant && !s.c.CommitShareToStorageRef {
		return res, nil
	}

	// TODO(labkode): if both commits are enabled they could be done concurrently.
	if s.c.CommitShareToStorageGrant {
		getShareReq := &usershareproviderv0alphapb.GetShareRequest{
			Ref: req.Ref,
		}
		getShareRes, err := c.GetShare(ctx, getShareReq)
		if err != nil {
			return nil, errors.Wrap(err, "gatewaysvc: error calling GetShare")
		}

		if getShareRes.Status.Code != rpcpb.Code_CODE_OK {
			res := &usershareproviderv0alphapb.UpdateShareResponse{
				Status: status.NewInternal(ctx, "error getting share when committing to the share"),
			}
			return res, nil
		}

		grantReq := &storageproviderv0alphapb.UpdateGrantRequest{
			Ref: &storageproviderv0alphapb.Reference{
				Spec: &storageproviderv0alphapb.Reference_Id{
					Id: getShareRes.Share.ResourceId,
				},
			},
			Grant: &storageproviderv0alphapb.Grant{
				Grantee:     getShareRes.Share.Grantee,
				Permissions: getShareRes.Share.Permissions.Permissions,
			},
		}
		grantRes, err := s.UpdateGrant(ctx, grantReq)
		if err != nil {
			return nil, errors.Wrap(err, "gatewaysvc: error calling UpdateGrant")
		}
		if grantRes.Status.Code != rpcpb.Code_CODE_OK {
			res := &usershareproviderv0alphapb.UpdateShareResponse{
				Status: status.NewInternal(ctx, "error updating storage grant"),
			}
			return res, nil
		}
	}

	return res, nil
}

func (s *svc) ListReceivedShares(ctx context.Context, req *usershareproviderv0alphapb.ListReceivedSharesRequest) (*usershareproviderv0alphapb.ListReceivedSharesResponse, error) {
	log := appctx.GetLogger(ctx)

	c, err := pool.GetUserShareProviderClient(s.c.UserShareProviderEndpoint)
	if err != nil {
		log.Err(err).Msg("gatewaysvc: error getting usershareprovider client")
		return &usershareproviderv0alphapb.ListReceivedSharesResponse{
			Status: &rpcpb.Status{
				Code: rpcpb.Code_CODE_INTERNAL,
			},
		}, nil
	}

	res, err := c.ListReceivedShares(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "gatewaysvc: error calling ListReceivedShares")
	}

	return res, nil
}

func (s *svc) GetReceivedShare(ctx context.Context, req *usershareproviderv0alphapb.GetReceivedShareRequest) (*usershareproviderv0alphapb.GetReceivedShareResponse, error) {
	log := appctx.GetLogger(ctx)

	c, err := pool.GetUserShareProviderClient(s.c.UserShareProviderEndpoint)
	if err != nil {
		log.Err(err).Msg("gatewaysvc: error getting usershareprovider client")
		return &usershareproviderv0alphapb.GetReceivedShareResponse{
			Status: &rpcpb.Status{
				Code: rpcpb.Code_CODE_INTERNAL,
			},
		}, nil
	}

	res, err := c.GetReceivedShare(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "gatewaysvc: error calling GetReceivedShare")
	}

	return res, nil
}

func (s *svc) UpdateReceivedShare(ctx context.Context, req *usershareproviderv0alphapb.UpdateReceivedShareRequest) (*usershareproviderv0alphapb.UpdateReceivedShareResponse, error) {
	log := appctx.GetLogger(ctx)

	c, err := pool.GetUserShareProviderClient(s.c.UserShareProviderEndpoint)
	if err != nil {
		log.Err(err).Msg("gatewaysvc: error getting usershareprovider client")
		return &usershareproviderv0alphapb.UpdateReceivedShareResponse{
			Status: &rpcpb.Status{
				Code: rpcpb.Code_CODE_INTERNAL,
			},
		}, nil
	}

	res, err := c.UpdateReceivedShare(ctx, req)
	if err != nil {
		log.Err(err).Msg("gateway: error calling UpdateReceivedShare")
		return &usershareproviderv0alphapb.UpdateReceivedShareResponse{
			Status: &rpcpb.Status{
				Code: rpcpb.Code_CODE_INTERNAL,
			},
		}, nil
	}

	if !s.c.CommitShareToStorageRef {
		return res, nil
	}

	if res.Status.Code != rpcpb.Code_CODE_OK {
		return res, nil
	}

	// we don't commit to storage invalid update fields.
	if req.Field.GetState() == usershareproviderv0alphapb.ShareState_SHARE_STATE_INVALID && req.Field.GetDisplayName() == "" {
		return res, nil

	}

	// TODO(labkode): if update field is displayName we need to do a rename on the storage to align
	// share display name and storage filename.
	if req.Field.GetState() != usershareproviderv0alphapb.ShareState_SHARE_STATE_INVALID {
		if req.Field.GetState() == usershareproviderv0alphapb.ShareState_SHARE_STATE_ACCEPTED {
			// get received share information to obtain the resource it points to.
			getShareReq := &usershareproviderv0alphapb.GetReceivedShareRequest{Ref: req.Ref}
			getShareRes, err := s.GetReceivedShare(ctx, getShareReq)
			if err != nil {
				log.Err(err).Msg("gateway: error calling GetReceivedShare")
				return &usershareproviderv0alphapb.UpdateReceivedShareResponse{
					Status: &rpcpb.Status{
						Code: rpcpb.Code_CODE_INTERNAL,
					},
				}, nil
			}

			if getShareRes.Status.Code != rpcpb.Code_CODE_OK {
				log.Error().Msg("gateway: error calling GetReceivedShare")
				return &usershareproviderv0alphapb.UpdateReceivedShareResponse{
					Status: &rpcpb.Status{
						Code: rpcpb.Code_CODE_INTERNAL,
					},
				}, nil
			}

			share := getShareRes.Share

			// get user home
			storageRegClient, err := pool.GetStorageRegistryClient(s.c.StorageRegistryEndpoint)
			if err != nil {
				log.Err(err).Msg("gatewaysvc: error getting storage registry client")
				return &usershareproviderv0alphapb.UpdateReceivedShareResponse{
					Status: &rpcpb.Status{
						Code: rpcpb.Code_CODE_INTERNAL,
					},
				}, nil
			}

			homeReq := &storageregv0alpahpb.GetHomeRequest{}
			homeRes, err := storageRegClient.GetHome(ctx, homeReq)
			if err != nil {
				log.Err(err).Msg("gateway: error calling GetHome")
				return &usershareproviderv0alphapb.UpdateReceivedShareResponse{
					Status: &rpcpb.Status{
						Code: rpcpb.Code_CODE_INTERNAL,
					},
				}, nil
			}
			
			// reference path is the home path + some name
			refPath := path.Join(homeRes.Path, req.Ref.String()) // TODO(labkode): the name of the share should be the filename it points to by default.
			createRefReq := &storageproviderv0alphapb.CreateReferenceRequest{
				Path:      refPath,
 				TargetUri: fmt.Sprintf("cs3:%s", req.Ref.GetId().OpaqueId),
			}
		}
	}
}
