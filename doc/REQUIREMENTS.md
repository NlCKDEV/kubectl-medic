# Functional Requirements

## 1. General
- Must run as `kubectl medic`.
- Must load kubeconfig & context automatically.
- UI must respond without blocking for API calls.

## 2. Namespace Handling
- List namespaces with filter and sorting.
- Display: name, status, age.
- Shortcut to health summary.

## 3. Pod Handling
- List pods with table columns:
  - name
  - restarts
  - ready count
  - status
  - age
- Sorting + filtering required.

## 4. Pod Detail View
- Show:
  - metadata
  - container list
  - container states
  - conditions
  - events
- Display recommended kubectl commands.

## 5. Diagnostics Engine
- Detect common issues:
  - Pending pod scheduling failures
  - Crash loops
  - ImagePull errors
  - Readiness/liveness probe issues
  - Resource quota issues
- Produce structured human-readable hints.

## 6. Logs Viewer
- Scrollable log screen.
- Multiple container support.

## 7. UI Requirements
- Uniform theme (Lip Gloss).
- 2–3 keypress rule for common actions.
- Help popup with all keybindings.

---

# Non-Functional Requirements

- Fast response (<300ms for UI updates).
- Safe: mainly read-only.
- Helpful error messages for RBAC issues.
- Compatible with Kubernetes ≥1.20.
