package publisher

import "context"

// Plan declares the existing Instance and public publication roots.
// It does not authorize publication or describe a CLI format.
type Plan struct{ InstanceRoot, Root string }

// Publication retains the genuine domain Publisher and the joined lifetime
// of its Source, role selections, Instance, history and optional parents.
// Owner lends the domain operations until Close joins the complete composed
// lifetime. The borrow does not transfer termination of the domain owner.
type Publication struct {
	owner Control
	close func() error
	serve func(context.Context) error
}

// Close joins all original physical and private borrowers before releasing roots.
func (p Publication) Close() error {
	if p.close == nil {
		return nil
	}
	return p.close()
}

// Serve transfers each accepted stream to the original qualified Publisher worker.
func (p Publication) Serve(ctx context.Context) error { return p.serve(ctx) }

// Owner borrows the genuine domain Publisher; the composition remains responsible
// for joining it together with its private roots and physical parents.
func (p Publication) Owner() Control { return p.owner }

// Control lends only the Publication operations used by the command. The
// concrete native Publisher is constructed only by Open on supported platforms;
// unsupported composition cannot make native domain effects reachable.
type Control interface {
	Publish(context.Context) (string, error)
	Refresh(context.Context) (string, error)
	Link(context.Context) (string, error)
	Withdraw(context.Context) error
}
