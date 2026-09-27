package mail

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const deliveryPressureWindow = 15 * time.Minute

func externalRecipientDomain(address string) (string, error) {
	normalized, err := NormalizeExternalAddress(address)
	if err != nil {
		return "", err
	}
	at := strings.LastIndexByte(normalized, '@')
	if at < 0 || at == len(normalized)-1 {
		return "", ErrInvalid
	}
	return strings.ToLower(normalized[at+1:]), nil
}

func deliveryPressureCooldown(transientFailures int) time.Duration {
	switch {
	case transientFailures >= 10:
		return 30 * time.Minute
	case transientFailures >= 6:
		return 15 * time.Minute
	case transientFailures >= 3:
		return 5 * time.Minute
	default:
		return 0
	}
}

// recordDomainDeliveryOutcomeTx updates only bounded aggregate state. Permanent
// recipient failures do not cool down a whole remote domain because they are
// normally address-specific rather than evidence that the domain is degraded.
func recordDomainDeliveryOutcomeTx(ctx context.Context, tx pgx.Tx, address string, class DeliveryFailureClass, delivered bool, now time.Time) error {
	domain, err := externalRecipientDomain(address)
	if err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var transientFailures, hardFailures, successes int
	var windowStarted time.Time
	var cooldownUntil *time.Time
	err = tx.QueryRow(ctx, `SELECT transient_failures,hard_failures,successes,window_started_at,cooldown_until
FROM mail_domain_delivery_pressure WHERE domain=$1 FOR UPDATE`, domain).Scan(&transientFailures, &hardFailures, &successes, &windowStarted, &cooldownUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		transientFailures, hardFailures, successes = 0, 0, 0
		windowStarted = now
		cooldownUntil = nil
	} else if err != nil {
		return err
	}

	if now.Sub(windowStarted) >= deliveryPressureWindow || now.Before(windowStarted) {
		transientFailures, hardFailures, successes = 0, 0, 0
		windowStarted = now
		if cooldownUntil != nil && !cooldownUntil.After(now) {
			cooldownUntil = nil
		}
	}

	if delivered {
		successes++
	} else {
		switch class {
		case FailureTransient:
			transientFailures++
			if d := deliveryPressureCooldown(transientFailures); d > 0 {
				candidate := now.Add(d)
				if cooldownUntil == nil || candidate.After(*cooldownUntil) {
					cooldownUntil = &candidate
				}
			}
		case FailureHard:
			hardFailures++
		}
	}

	_, err = tx.Exec(ctx, `INSERT INTO mail_domain_delivery_pressure(domain,transient_failures,hard_failures,successes,window_started_at,cooldown_until,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT(domain) DO UPDATE SET transient_failures=EXCLUDED.transient_failures,hard_failures=EXCLUDED.hard_failures,
 successes=EXCLUDED.successes,window_started_at=EXCLUDED.window_started_at,cooldown_until=EXCLUDED.cooldown_until,updated_at=EXCLUDED.updated_at`,
		domain, transientFailures, hardFailures, successes, windowStarted, cooldownUntil, now)
	return err
}

func activeDomainCooldownTx(ctx context.Context, tx pgx.Tx, address string, now time.Time) (*time.Time, error) {
	domain, err := externalRecipientDomain(address)
	if err != nil {
		return nil, err
	}
	var until *time.Time
	err = tx.QueryRow(ctx, `SELECT cooldown_until FROM mail_domain_delivery_pressure WHERE domain=$1`, domain).Scan(&until)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if until == nil || !until.After(now) {
		return nil, nil
	}
	return until, nil
}
