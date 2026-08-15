# Service command end to end

This test builds `ardents-service`, starts it with a valid endpoint plan, observes
its public readiness event, lets the application-accept deadline expire, and
asserts process failure plus socket cleanup. Exact-Target transfer and
same-connection recovery stay at the `internal/serviceconn` Module seam until a
maintained Route Attachment command exists.
