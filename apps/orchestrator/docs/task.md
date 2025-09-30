# Rules
* Do not modify the `pipeline` and `bilevel` packages.
* Adhere to the `bilevel` package contract. `State` and `Adapter` methods do not need to be thread-safe, as they are called from a single goroutine. However, you must manage their state carefully, as the method call order (e.g., `Update` vs. `Next`) is unpredictable.

# Your Task
