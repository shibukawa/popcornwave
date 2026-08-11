# fastpages

A page tree with no server action, used to generate the same tree for both
transports in one test.

It is separate from `pages` because that tree declares a server action, and an
action handler is written for one transport: the net/http fixture's `Rename`
takes a writer and a request, which the fasthttp handler-shape recognizer
correctly refuses. Generating the second transport's tree from the first
transport's source is not a real scenario — the transform rewrites the handler
first — so the shared fixture is one that declares no handler at all.
