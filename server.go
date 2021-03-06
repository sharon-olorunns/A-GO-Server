package server

import (
	"fmt"
	"net/http"
  "os"
  "crypto/rand"
	"io"
	"math/big"
)

// primeHandler responds with a vanity prime
func primeHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	vals := query["vs"]
	if len(vals) > 0 {
		fmt.Fprintf(w, getVanityPrime(vals[0]))
	} else {
		fmt.Fprintf(w, "no vs param")
	}
}

// exitHandler implements the exit program ability
func exitHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "exiting")
	go exit()
}

// Init starts the http server on the given port
func Init(port string) {
	// register route handler
	http.HandleFunc("/.well-known/vanityprime", primeHandler)
	http.HandleFunc("/.well-known/vpexit", exitHandler)

	// start listening
	http.ListenAndServe(port, nil)
}

// exit exits the current program
func exit() {
	os.Exit(0)
}

// getVanityPrime generates the vanity prime
func getVanityPrime(vanity string) string {
	p, err := vanityPrime(vanity)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return p.Text(16)
}

// smallPrimes is a list of small, prime numbers that allows us to rapidly
// exclude some fraction of composite candidates when searching for a random
// prime. This list is truncated at the point where smallPrimesProduct exceeds
// a uint64. It does not include two because we ensure that the candidates are
// odd by construction.
var smallPrimes = []uint8{
	3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47, 53,
}

// smallPrimesProduct is the product of the values in smallPrimes and allows us
// to reduce a candidate prime by this number and then determine whether it's
// coprime to all the elements of smallPrimes without further big.Int
// operations.
var smallPrimesProduct = new(big.Int).SetUint64(16294579238595022365)

// bits defines the number of bits in the generated prime number
const bits = 1024

// vanityPrime returns a number, p, of the given size, such that p is prime
// with high probability.
// vanityPrime will return error for any error returned by rand.Read
func vanityPrime(vanity string) (p *big.Int, err error) {
	b := uint(8)
	bytes := make([]byte, 128)
	p = new(big.Int)

	bigMod := new(big.Int)

	for {
		_, err = io.ReadFull(rand.Reader, bytes)
		if err != nil {
			return nil, err
		}

		// Clear bits in the first byte to make sure the candidate has a size <= bits.
		bytes[0] &= uint8(int(1<<b) - 1)
		// Don't let the value be too small, i.e, set the most significant two bits.
		// Setting the top two bits, rather than just the top bit,
		// means that when two of these values are multiplied together,
		// the result isn't ever one bit short.
		if b >= 2 {
			bytes[0] |= 3 << (b - 2)
		} else {
			// Here b==1, because b cannot be zero.
			bytes[0] |= 1
			if len(bytes) > 1 {
				bytes[1] |= 0x80
			}
		}
		// Make the value odd since an even number this large certainly isn't prime.
		bytes[len(bytes)-1] |= 1

		p.SetBytes(bytes)

		// Calculate the value mod the product of smallPrimes. If it's
		// a multiple of any of these primes we add two until it isn't.
		// The probability of overflowing is minimal and can be ignored
		// because we still perform Miller-Rabin tests on the result.
		bigMod.Mod(p, smallPrimesProduct)
		mod := bigMod.Uint64()

	NextDelta:
		for delta := uint64(0); delta < 1<<20; delta += 2 {
			m := mod + delta
			for _, prime := range smallPrimes {
				if m%uint64(prime) == 0 && (bits > 6 || m != uint64(prime)) {
					continue NextDelta
				}
			}

			if delta > 0 {
				bigMod.SetUint64(delta)
				p.Add(p, bigMod)
			}
			break
		}

		// There is a tiny possibility that, by adding delta, we caused
		// the number to be one bit too long. Thus we check BitLen
		// here.
		if p.ProbablyPrime(20) && p.BitLen() == bits {
			return
		}
	}
}
