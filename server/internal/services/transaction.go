package services

import "context"

// Transactor is the service-facing contract. Database and ORM details stay in db.
type Transactor interface {
	WithTransaction(context.Context, func(context.Context) error) error
}
