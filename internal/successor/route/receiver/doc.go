// Package receiver owns physical receiving composition: listener, exact-purpose
// dispatch, directed Node Carrier borrowing and joined connection retirement.
// Actual Admission grants and transferred Hosting reservations remain held until
// all physical borrowers join. Introduction and JOIN own their handler state;
// receiver owns no token quota, spend policy, selection or holder generation.
// Native receiving and Introduction root adapters currently require Linux.
package receiver
