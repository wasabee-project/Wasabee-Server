package model

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// LinkID wrapper to ensure type safety
type LinkID string

// Link is the basic link data structure
type Link struct {
	ID         LinkID   `json:"ID"`
	From       PortalID `json:"fromPortalId"`
	To         PortalID `json:"toPortalId"`
	Desc       string   `json:"description"` // deprecated, use Comment from Task
	AssignedTo GoogleID `json:"assignedTo"`  // deprecated, use Assignments from Task
	Color      string   `json:"color"`
	Task
	MuCaptured int   `json:"mu"`
	ThrowOrder int16 `json:"throwOrderPos"` // deprecated, use Order from Task
	Completed  bool  `json:"completed"`     // deprecated, use State from Task
}

// insertLink adds a link and its underlying task to the database
func (opID OperationID) insertLink(ctx context.Context, l Link, tx *sql.Tx) error {
	if l.To == l.From {
		log.Infow("source and destination the same, ignoring link", "resource", opID)
		return nil
	}

	if !l.Zone.Valid() || l.Zone == ZoneAll {
		l.Zone = zonePrimary
	}

	// Legacy field sync
	if l.Desc != "" {
		l.Comment = l.Desc
	}
	if l.ThrowOrder != 0 {
		l.Order = l.ThrowOrder
	}
	if l.AssignedTo != "" {
		l.Assignments = append(l.Assignments, l.AssignedTo)
	}
	if l.State == "" {
		l.State = "pending"
	}
	if l.Completed {
		l.State = "completed"
	}

	comment := makeNullString(util.Sanitize(l.Comment))
	executor := txExecutor(tx)

	// 1. Insert Task
	_, err := executor.ExecContext(ctx, "INSERT INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		l.ID, opID, comment, l.Order, l.State, l.Zone, l.DeltaMinutes)
	if err != nil {
		return err
	}

	// 2. Insert Link
	_, err = executor.ExecContext(ctx, "INSERT INTO link (ID, opID, fromPortalID, toPortalID, color, mu) VALUES (?, ?, ?, ?, ?, ?)",
		l.ID, opID, l.From, l.To, l.Color, l.MuCaptured)
	if err != nil {
		return err
	}

	// Initialize Task fields for method calls
	l.Task.ID = TaskID(l.ID)
	l.Task.opID = opID

	if err := l.SetAssignments(ctx, l.Assignments, tx); err != nil {
		return err
	}

	if len(l.DependsOn) > 0 {
		return l.SetDepends(ctx, l.DependsOn, tx)
	}

	return nil
}

func (opID OperationID) updateLink(ctx context.Context, l Link, tx *sql.Tx) error {
	if l.To == l.From {
		return nil
	}

	l.Task.ID = TaskID(l.ID)
	l.Task.opID = opID

	if !l.Zone.Valid() || l.Zone == ZoneAll {
		l.Zone = zonePrimary
	}

	// Legacy field sync
	if l.Desc != "" {
		l.Comment = l.Desc
	}
	if l.ThrowOrder != 0 {
		l.Order = l.ThrowOrder
	}
	if l.AssignedTo != "" {
		l.Assignments = append(l.Assignments, l.AssignedTo)
	}
	if l.Completed {
		l.State = "completed"
	} else if l.State == "" {
		l.State = "pending"
	}

	comment := makeNullString(util.Sanitize(l.Comment))
	executor := txExecutor(tx)

	// Update Task
	_, err := executor.ExecContext(ctx, "INSERT INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE comment = ?, taskorder = ?, state = ?, zone = ?, delta = ?",
		l.ID, opID, comment, l.Order, l.State, l.Zone, l.DeltaMinutes,
		comment, l.Order, l.State, l.Zone, l.DeltaMinutes)
	if err != nil {
		return err
	}

	// Update Link
	_, err = executor.ExecContext(ctx, "REPLACE INTO link (ID, opID, fromPortalID, toPortalID, color, mu) VALUES (?, ?, ?, ?, ?, ?)",
		l.ID, opID, l.From, l.To, l.Color, l.MuCaptured)
	if err != nil {
		return err
	}

	if err := l.SetAssignments(ctx, l.Assignments, tx); err != nil {
		return err
	}

	if len(l.DependsOn) > 0 {
		return l.SetDepends(ctx, l.DependsOn, tx)
	}

	return nil
}

func (opID OperationID) deleteLink(ctx context.Context, lid LinkID, tx *sql.Tx) error {
	executor := txExecutor(tx)
	_, err := executor.ExecContext(ctx, "DELETE FROM task WHERE OpID = ? and ID = ?", opID, lid)
	return err
}

// populateLinks fills in the Links list for the Operation.
func (o *Operation) populateLinks(ctx context.Context, zones []ZoneID, inGid GoogleID, assignments map[TaskID][]GoogleID, depends map[TaskID][]TaskID) error {
	rows, err := db.QueryContext(ctx, "SELECT link.ID, link.fromPortalID, link.toPortalID, task.comment, task.taskorder, task.state, link.color, task.zone, task.delta, link.mu FROM link JOIN task ON link.ID = task.ID WHERE task.opID = ? AND link.opID = task.opID", o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tmpLink Link
		tmpLink.opID = o.ID
		var comment sql.NullString

		err := rows.Scan(&tmpLink.ID, &tmpLink.From, &tmpLink.To, &comment, &tmpLink.Order, &tmpLink.State, &tmpLink.Color, &tmpLink.Zone, &tmpLink.DeltaMinutes, &tmpLink.MuCaptured)
		if err != nil {
			continue
		}

		tmpLink.Task.ID = TaskID(tmpLink.ID)
		tmpLink.ThrowOrder = tmpLink.Order

		if comment.Valid {
			tmpLink.Comment = comment.String
			tmpLink.Desc = comment.String
		}

		if a, ok := assignments[tmpLink.Task.ID]; ok {
			tmpLink.Assignments = a
			tmpLink.AssignedTo = a[0]
		}
		if d, ok := depends[tmpLink.Task.ID]; ok {
			tmpLink.DependsOn = d
		}

		tmpLink.Completed = (tmpLink.State == "completed")

		if !tmpLink.Zone.inZones(zones) && !tmpLink.IsAssignedTo(ctx, inGid) {
			continue
		}
		o.Links = append(o.Links, tmpLink)
	}
	return nil
}

func (l LinkID) String() string { return string(l) }

// LinkOrder updates the taskorder field in the task table
func (o *Operation) LinkOrder(ctx context.Context, order string) error {
	links := strings.Split(order, ",")
	for pos, lid := range links {
		if lid == "000" {
			continue
		}
		if _, err := db.ExecContext(ctx, "UPDATE task SET taskorder = ? WHERE opID = ? AND ID = ?", pos+1, o.ID, lid); err != nil {
			log.Error(err)
		}
	}
	return nil
}

// SetColor changes the color of a link
func (l *Link) SetColor(ctx context.Context, color string) error {
	_, err := db.ExecContext(ctx, "UPDATE link SET color = ? WHERE ID = ? and opID = ?", color, l.ID, l.opID)
	return err
}

// Swap flips the direction of a link
func (l *Link) Swap(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "UPDATE link SET fromPortalID = toPortalID, toPortalID = fromPortalID WHERE ID = ? and opID = ?", l.ID, l.opID)
	return err
}

// GetLink returns a link from the operation cache
func (o *Operation) GetLink(linkID LinkID) (*Link, error) {
	for _, l := range o.Links {
		if l.ID == linkID {
			return &l, nil
		}
	}
	return nil, errors.New(ErrLinkNotFound)
}
