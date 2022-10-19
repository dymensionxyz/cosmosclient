# Cosmos Client

A client to access the settlement-chain (or any other chain) by querying and broadcasting transactions.

This client cloned from [ignite's cosmosclient](https://github.com/ignite/cli/tree/develop/ignite/pkg/cosmosclient) and it's advantage is that it doesn't use the `cosmos-sdk` config singleton.

This config-singleton use several chain specific properties (like address-prefix and coin-type) and it is impossible to override these properties (using the already-made `cosmosclient`).

So this tool give the option the use `cosmosclient` for other chain without using the chain-specific properties supplied by the cosmos-sdk config.
