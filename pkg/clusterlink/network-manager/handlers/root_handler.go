package handlers

type RootHandler struct {
	Next
}

func (h *RootHandler) Do(_ *Context) (err error) {
	return
}

type Handler interface {
	Do(c *Context) error
	SetNext(handler Handler) Handler
	Run(c *Context) error
}

type Next struct {
	nextHandler Handler
}

func (n *Next) SetNext(handler Handler) Handler {
	n.nextHandler = handler
	return handler
}

func (n *Next) Run(c *Context) (err error) {
	if n.nextHandler != nil {
		if err = (n.nextHandler).Do(c); err != nil {
			return
		}
		return (n.nextHandler).Run(c)
	}
	return
}
