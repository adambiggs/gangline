package tmux

import (
	"context"
	"fmt"
	"strings"
)

const widgetSuffix = " #{@gangline_context_value}"

func (backend *Backend) sessionOptions(ctx context.Context) (string, error) {
	out, err := backend.run(ctx, "show-options", "-t", backend.config.Session)
	if err != nil {
		return "", tmuxError("read session options", err, out)
	}
	return out, nil
}
func (backend *Backend) sessionOption(ctx context.Context, name string) (string, error) {
	out, err := backend.run(ctx, "show-options", "-Av", "-t", backend.config.Session, name)
	if err != nil {
		return "", tmuxError("read session option", err, out)
	}
	return strings.TrimSuffix(out, "\n"), nil
}
func (backend *Backend) setSessionOption(ctx context.Context, name, value string) error {
	out, err := backend.run(ctx, "set-option", "-t", backend.config.Session, name, value)
	if err != nil {
		return tmuxError("set session option", err, out)
	}
	return nil
}
func (backend *Backend) unsetSessionOption(ctx context.Context, name string) error {
	out, err := backend.run(ctx, "set-option", "-u", "-t", backend.config.Session, name)
	if err != nil {
		return tmuxError("restore session option", err, out)
	}
	return nil
}
func hasSessionOption(options, name string) bool {
	for _, line := range strings.Split(options, "\n") {
		if strings.HasPrefix(line, name+" ") {
			return true
		}
	}
	return false
}

// ContextWidget changes this team's session options only. The original local
// value (or inheritance) is restored on disable, unless the operator changed it.
func (backend *Backend) ContextWidget(ctx context.Context, hitch, value string) error {
	options, err := backend.sessionOptions(ctx)
	if err != nil {
		return err
	}
	enabled := hasSessionOption(options, "@gangline_context_hitch")
	if hitch == "" {
		if !enabled {
			return nil
		}
		original, err := backend.sessionOption(ctx, "@gangline_context_original")
		if err != nil {
			return err
		}
		current, err := backend.sessionOption(ctx, "status-right")
		if err != nil {
			return err
		}
		if current != original+widgetSuffix {
			return fmt.Errorf("session status-right changed since widget enable; refusing to overwrite operator changes")
		}
		local, err := backend.sessionOption(ctx, "@gangline_context_local")
		if err != nil {
			return err
		}
		if local == "true" {
			err = backend.setSessionOption(ctx, "status-right", original)
		} else {
			err = backend.unsetSessionOption(ctx, "status-right")
		}
		if err != nil {
			return err
		}
		for _, name := range []string{"@gangline_context_hitch", "@gangline_context_original", "@gangline_context_local", "@gangline_context_value"} {
			if err := backend.unsetSessionOption(ctx, name); err != nil {
				return err
			}
		}
		return nil
	}
	if !enabled {
		original, err := backend.sessionOption(ctx, "status-right")
		if err != nil {
			return err
		}
		for _, pair := range [][2]string{{"@gangline_context_original", original}, {"@gangline_context_local", fmt.Sprint(hasSessionOption(options, "status-right"))}, {"status-right", original + widgetSuffix}} {
			if err := backend.setSessionOption(ctx, pair[0], pair[1]); err != nil {
				return err
			}
		}
	}
	if err := backend.setSessionOption(ctx, "@gangline_context_value", value); err != nil {
		return err
	}
	return backend.setSessionOption(ctx, "@gangline_context_hitch", hitch)
}

func (backend *Backend) PublishContext(ctx context.Context, hitch, value string) error {
	exists, err := backend.SessionExists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	options, err := backend.sessionOptions(ctx)
	if err != nil {
		return err
	}
	if !hasSessionOption(options, "@gangline_context_hitch") {
		return nil
	}
	target, err := backend.sessionOption(ctx, "@gangline_context_hitch")
	if err != nil {
		return err
	}
	if target != hitch {
		return nil
	}
	return backend.setSessionOption(ctx, "@gangline_context_value", value)
}
