# Locks

- DirectReleaseBoundary maps ExplicitInput for ReleaseBaseline, TaskPublication, RustSuite, Export, and BuildInstall to TypedResult without shared lock.

## CargoLock

```
ObservableCargoLockWaitOwner
+-- Consumer: RustSuiteLockAcquisitionConsumer
+-- Evidence: LockRecord->flock->LockGroupRecord->payload
+-- Successor: BoundedCargoLockWaitSuccessor
`-- Terminal: TypedCargoLockWaitTerminal
```

- Fixed no-follow CargoLock is `$PROJECT_SCRATCH_ROOT/locks/cargo-lock` outside source, export, record, attempt roots.
- Baseline and other consumers retain blocking lock spans.
- RustSuite tries nonblocking once and holds it through execution.
- Contention/cancellation yields Stopped Helper before Plan/product.
- Absent: polling, ledger, deletion, registry, retirement, cleanup, topology lock, journal.
