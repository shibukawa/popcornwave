---
id: decision:cache-key-interface
type: decision
title: A Cache Key Is An Interface, Not A Shape
---
The key type of requirement:data-result-cache satisfies a one-method interface, and generation writes that method from struct tags rather than being the mechanism that derives a key.

```yaml
status: accepted 2026-08-13 by the owner, who raised the struct-or-map shape and withdrew it; the interface and its generator both ship, the latter as cachekeybind in system:tinybind v0.5.9
owner: api:data-cache
interface: one method returning the framed key string, implemented by the key type
rejected_shape:
  proposal: accept either a tagged struct or a map, deriving the key from whichever arrived
  why_it_fails:
    no_constraint_expresses_it: struct-or-map forces the type parameter to any, and the derivation then type-switches at run time
    which_is_reflection: forbidden by api:typed-external-function and by the generated-mapping constraint of system:tinybind, in both cases because the framework does not walk a value it did not generate code for
    two_failure_modes_on_one_signature: the struct path fails at build on a missing tag, the map path fails at run time on an unsupported value type, and which one a call site gets depends on its type argument
    map_ordering_is_the_small_problem: sorting keys is easy; the encoding must also distinguish a numeric one from a string one or two maps collide, which is a type marker the tagged path never needs
discovery:
  problem_with_the_call_site: a Memo call is ordinary Go in a hand-written file, so emitting an encoder per key type would mean discovering every instantiation of one generic function
  answer: discovery moves to the tag — the generator scans for types carrying cache tags and emits the method with a compile-time assertion against the interface
  precedent: the dynamo tag of system:tinybind already works this way, yielding EncodeItem, DecodeItem, and ItemKey per tagged type with the same assertion
  upstream_shape: a generator mode emitting one method from tags, which is smaller than the runtime key binder the same request first described
hand_written: always available, built from the exported htmlbind Key helpers, which is the escape hatch a genuinely dynamic key uses by sorting inside its own method
map_support_dropped:
  not_a_loss: a dynamic key set cannot be checked against the safety rule that the whole dependency set is in the key, and making that set visible is what the tag is for
  still_reachable: a named map type with a hand-written method, which puts the runtime failure in the application that chose it
identity_rides_along: decision:data-cache-entry-identity, since the method is where the key type's own name enters the key
```
