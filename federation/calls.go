package federation

import (
	"context"
	"strconv"

	pb "github.com/wasabee-project/Wasabee-Server/federation/pb"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func (w *wafed) SetCommunityID(ctx context.Context, in *pb.CommunityID) (*pb.Error, error) {
	var e pb.Error

	gid := model.GoogleID(in.Googleid)

	if err := gid.SetCommunityName(in.Communityname); err != nil {
		log.Error(err)
		e.Message = err.Error()
		return &e, err
	}

	e.Message = "ok"
	return &e, nil
}

func (w *wafed) SetAgentLocation(ctx context.Context, in *pb.AgentLocation) (*pb.Error, error) {
	var e pb.Error

	gid := model.GoogleID(in.Googleid)

	lat := strconv.FormatFloat(float64(in.Lat), 'f', 7, 32)
	lng := strconv.FormatFloat(float64(in.Lng), 'f', 7, 32)

	if err := gid.SetLocation(lat, lng); err != nil {
		log.Error(err)
		e.Message = err.Error()
		return &e, err
	}

	e.Message = "ok"
	return &e, nil
}

func (w *wafed) SetIntelData(ctx context.Context, in *pb.IntelData) (*pb.Error, error) {
	var e pb.Error

	gid := model.GoogleID(in.Googleid)

	if err := gid.SetIntelData(in.Name, in.Faction); err != nil {
		log.Error(err)
		e.Message = err.Error()
		return &e, err
	}

	e.Message = "ok"
	return &e, nil
}

func (w *wafed) AddFirebaseToken(ctx context.Context, in *pb.FBData) (*pb.Error, error) {
	var e pb.Error

	gid := model.GoogleID(in.Googleid)

	if err := gid.StoreFirebaseToken(in.Token); err != nil {
		log.Error(err)
		e.Message = err.Error()
		return &e, err
	}

	e.Message = "ok"
	return &e, nil
}
