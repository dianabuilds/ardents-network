// Package deployment validates bounded infrastructure deployment declarations.
// Its fixed Node path grammar targets the selected Linux installation while
// running on any host; it observes no filesystem. Public Network trust inputs
// are checked for shape only; Network State independently authenticates them.
// No declaration supplies Release, account, process or runtime permission.
package deployment
