package mailconn

import (
	"context"
	"errors"
	"fmt"

	"wmail-go/internal/sieve"
)

// DefaultSievePort est le port ManageSieve (RFC 5804).
const DefaultSievePort = 4190

// WithSieve ouvre une session ManageSieve (connexion protégée comme les autres, chiffrement selon e.Security,
// authentification si e.Login n'est pas vide), exécute fn puis la ferme. Les erreurs sont des *TestError.
func (g *Guard) WithSieve(ctx context.Context, e Endpoint, fn func(*sieve.Client) error) error {
	err := g.withSieve(ctx, e, fn)
	return g.legacyHint(err, e, func(e2 Endpoint) error { return g.withSieve(ctx, e2, func(*sieve.Client) error { return nil }) })
}

func (g *Guard) withSieve(ctx context.Context, e Endpoint, fn func(*sieve.Client) error) error {
	conn, stop, err := g.connect(ctx, e, e.Security == SecurityTLS)
	if err != nil {
		return classify(ctx, err)
	}
	defer stop()
	c, err := sieve.New(conn)
	if err != nil {
		conn.Close()
		return classify(ctx, err)
	}
	defer c.Close()
	if e.Security == SecuritySTARTTLS {
		if err := c.StartTLS(e.tlsConfig()); err != nil {
			return classify(ctx, err)
		}
	}
	if e.Login != "" {
		if err := c.Authenticate(e.Login, e.Password); err != nil {
			if errors.Is(err, sieve.ErrAuth) {
				return classify(ctx, &authStage{err})
			}
			return classify(ctx, err)
		}
	}
	if err := fn(c); err != nil {
		var se *sieve.ServerError
		if errors.As(err, &se) {
			return err // refus du serveur (script invalide, quota…) : l'appelant en affiche le texte
		}
		return classify(ctx, err)
	}
	_ = c.Logout()
	return nil
}

// TestSieve vérifie la connexion, le chiffrement et l'authentification ManageSieve.
func (g *Guard) TestSieve(ctx context.Context, e Endpoint) error {
	return g.WithSieve(ctx, e, func(*sieve.Client) error { return nil })
}

var _ = fmt.Sprint
