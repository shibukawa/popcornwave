---
id: requirement:dynamodb-page-cursor
type: requirement
title: DynamoDB Page Cursor
---
A page's last evaluated key becomes one opaque route parameter, so a next-page link is a link and no handler serializes an attribute map into a URL.

```yaml
audience: actor:application-developer
problem:
  what_the_driver_returns: a last evaluated key, which is an attribute map
  what_a_page_needs: one string that survives a URL, a form field, and a browser history entry
  today: every application writes the same encoding, and the naive version puts raw attribute values into a query string
  why_the_framework: this is where a store meets requirement:discovered-page-routing, and neither system:tinybind nor the driver is in that position
encoding:
  form: base64url of the key's JSON form, with no padding
  losslessness: measured upstream over a 38-digit number, high-byte binary, and a NUL-bearing multi-byte string, so no attribute type is deformed by the trip
  reason_json: the driver already marshals attribute values as JSON, so the cursor needs no second codec and cannot disagree with the first
  size: one key of at most two attributes, which stays inside a normal URL bound; a cursor that would not is an error rather than a truncated string
route_binding:
  parameter: an optional query input, which requirement:discovered-page-routing binds as a pointer so an absent cursor is distinguishable from an empty one
  first_page: no parameter
  next_page: the page renders the link only when the reply carried a last evaluated key
  invalid: a cursor that does not decode is a bad request before any DynamoDB call, never a server error
surface:
  encode: from a page returned by requirement:dynamodb-typed-queries, or from a raw driver key
  decode: to the driver option that resumes the query
  shape: two functions, because a cursor is a value and not a session
safety:
  the_claim_to_reject: a cursor is a position in a table, not a permission to read it
  what_actually_scopes_a_read: the key condition, which requirement:dynamodb-typed-queries fixes at generation and the handler parameterizes from its own context
  therefore: a forged cursor cannot reach a partition the key condition does not name, and the guarantee comes from the condition rather than from the cursor being unforgeable
  what_a_forged_cursor_can_do: skip forward within a partition the caller may already read, which is not a disclosure
  upstream_caution: system:tinybind warns that a signature must cover whatever scopes the query, which is exactly right and is why the scoping must stay in the key condition
  the_case_that_breaks_it:
    when: a query scoped by a filter expression rather than by its key, such as a tenant column that is not the partition key
    why: a filter is applied after the read, so a cursor plus a differently scoped call can start outside the intended range
    today: unreachable, because filters are out of scope for a declared query
    on_arrival: this requirement is revisited before generated filters ship, and a scope-bound signature is the answer if they scope anything
  not_signed: no signature and no framework secret, because none is needed for the reachable cases and inventing a secret to manage would be the larger cost
  disclosure: anyone may decode a cursor and read its key attribute values, so a key attribute that is itself a secret must not be paged over; the guide says so
  never: a cursor is not an authorization token, is not accepted in place of one, and carries no identity
acceptance:
  - a page with more results renders a next link, and following it returns the following items with no repeats and no gaps
  - the last page renders no link
  - a hand-edited cursor is rejected as a bad request without a DynamoDB call
  - a cursor survives a URL, a form post, and a browser back button unchanged
  - a key holding a 38-digit number, binary, and a multi-byte string round trips
  - no handler names an attribute in order to build or read a cursor
non_goals:
  - a page-number or total-count model, which DynamoDB does not answer
  - a previous-page link, since the service gives no backward cursor
  - server-side cursor storage
  - a cursor over a scan of a table the caller is not otherwise scoped to
```
