package asterdoc

import (
	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/uikit/kit"
)

type asterMenu = kit.Menu
type asterMenuItem = kit.MenuItem

func asterContextMenu(c *ui.Context, target ui.Element, build func(m *asterMenu)) {
	kit.ContextMenu(c, target, build)
}
