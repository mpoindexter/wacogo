;; WACOGO: edited error message
(assert_unlinkable
  (component
    (import "undefined-name" (core module))
  )
  "missing import named `undefined-name`")
(component $i)
(component
  (import "i" (instance))
)
;; WACOGO: edited error message to add comma
(assert_unlinkable
  (component (import "i" (core module)))
  "expected module, found instance")
;; WACOGO: edited error message to add comma, change type name
(assert_unlinkable
  (component (import "i" (func)))
  "expected func, found instance")
;; WACOGO: edited error message
(assert_unlinkable
  (component (import "i" (instance (export "x" (func)))))
  "missing expected export `x`")
