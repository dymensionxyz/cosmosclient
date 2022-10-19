# Cosmos Client

This client provides a standalone client to connect to Cosmos SDK chains from a codebase which already makes use of the cosmos-sdk.

The client was created due to the collision created by the cosmos-sdk config singleton as it only accepts (for example) single address-prefix and coin-type and thus prevents the usage of multiple configs.

This client is based on the work done on [ignite’s cosmosclient](https://github.com/ignite/cli/tree/develop/ignite/pkg/cosmosclient).
