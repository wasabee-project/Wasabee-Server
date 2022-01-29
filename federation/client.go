package federation

import (
	"context"

	pb "github.com/wasabee-project/Wasabee-Server/federation/pb"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func SetCommunityID(ctx context.Context, gid model.GoogleID, name string) error {
	for _, p := range peers {
		r, err := p.SetCommunityID(ctx, &pb.CommunityID{
			Googleid:      string(gid),
			Communityname: name,
		})
		if err != nil {
			log.Error(err)
			continue
		}
		log.Debugw("SetCommunityID", "r", r)
	}
	return nil
}

func SetAgentLocation(ctx context.Context, gid model.GoogleID, lat, lng float32) error {
	for _, p := range peers {
		r, err := p.SetAgentLocation(ctx, &pb.AgentLocation{
			Googleid: string(gid),
			Lat:      lat,
			Lng:      lng,
		})
		if err != nil {
			log.Error(err)
			continue
		}
		log.Debugw("SetAgentLocation", "r", r)
	}
	return nil
}

func SetIntelData(ctx context.Context, gid model.GoogleID, intelname, faction string) error {
	for _, p := range peers {
		r, err := p.SetIntelData(ctx, &pb.IntelData{
			Googleid: string(gid),
			Name:     intelname,
			Faction:  faction,
		})
		if err != nil {
			log.Error(err)
			continue
		}
		log.Debugw("SetIntelData", "r", r)
	}
	return nil
}

func AddFirebaseToken(ctx context.Context, gid model.GoogleID, token string) error {
	for _, p := range peers {
		r, err := p.AddFirebaseToken(ctx, &pb.FBData{
			Googleid: string(gid),
			Token:    token,
		})
		if err != nil {
			log.Error(err)
			continue
		}
		log.Debugw("AddFirebaseToken", "r", r)
	}
	return nil
}
