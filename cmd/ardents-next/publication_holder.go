package main

import "context"

type publicationPlan struct {
	InstanceRoot string `json:"instance_root"`
	Root         string `json:"root"`
}

type publicationHandle struct {
	owner interface {
		Publish(context.Context) (string, error)
		Refresh(context.Context) (string, error)
		Link(context.Context) (string, error)
		Withdraw(context.Context) error
	}
	close func() error
}

func (holder publicationHandle) publish(ctx context.Context) (string, error) {
	return holder.owner.Publish(ctx)
}

func (holder publicationHandle) refresh(ctx context.Context) (string, error) {
	return holder.owner.Refresh(ctx)
}

func (holder publicationHandle) link(ctx context.Context) (string, error) {
	return holder.owner.Link(ctx)
}

func (holder publicationHandle) withdraw(ctx context.Context) error {
	return holder.owner.Withdraw(ctx)
}
