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
		log.Debug("SetCommunityID", "r", r)
	}
	return nil
}
