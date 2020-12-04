# Availability Zones

The desired Availability Zones are defined in the `MachinePool` CR.
This test fetches one of the cluster's `MachinePool` objects to see what are the expected AZs.
Then it reads the actual used AZs by the VMSS on Azure. 
