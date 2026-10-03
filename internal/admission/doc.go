// Package admission owns the signed offline closed-admission grammar: the
// sealed 228-byte holder permission allocation, the 269-byte holder-signed
// allocation request, and the finite Node-signed closed issuer public key
// inventory (ARDCIP01). Every codec here is exact-bytes canonical, offline
// and standard-library-only apart from State's closed SPKI verification; the
// package grants no admission authority, selects no route and opens no
// network listener. Token and issuer subpackages consume this grammar for
// holder/receiver operations and live issuing respectively. Offline Control
// and Custody commands depend on this package without importing the live
// listener (F-28/F-30 seam).
package admission
