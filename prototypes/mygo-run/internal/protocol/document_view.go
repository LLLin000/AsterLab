package protocol

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// RenderMarkdown renders a Markdown document with the same reading typography
// as the frozen protocol. The caller owns the surface and the document content.
func RenderMarkdown(c *ui.Context, text string) {
	markdown.View(c, text, &markdown.Options{Theme: markdownTheme(c.Theme())})
}

// RenderMarkdownWith renders Markdown with the reading theme and lets the
// caller draw fenced code blocks whose language it owns.
func RenderMarkdownWith(c *ui.Context, text string, code func(*ui.Context, *markdown.CodeBlockView)) {
	options := &markdown.Options{Theme: markdownTheme(c.Theme())}
	if code != nil {
		options.CodeBlock = code
	}
	markdown.View(c, text, options)
}

// ViewDocument renders the frozen protocol as a reading page: MyGo's markdown
// view, themed with AsterLab's tokens, on one paper surface. The projection owns
// every word; the view adds no content of its own.
func ViewDocument(c *ui.Context, doc Document) {
	t := c.Theme()
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(t.Space(7), t.Space(6)).AlignItems(ui.Center).
			Children(func() {
				ui.Box(c).FillWidth().MaxWidth(820).MaxWidthPercent(100).
					Radius(float32(18)).Border(1, t.Border).Background(t.Surface).
					Padding(t.Space(8), t.Space(9)).Children(func() {
					markdown.View(c, doc.Markdown(), &markdown.Options{Theme: markdownTheme(t)})
				})
			})
	})
}
