# Use Cases

## UC1 – Browse Namespaces Quickly
- User opens tool → sees a scrollable namespace list.
- User filters with `/`.
- User selects namespace → pod list loads instantly.

## UC2 – Identify Failing Pods
- Pods list shows problematic pods highlighted.
- User presses `x` to run diagnostics.
- System displays interpreted suggestions:
  - CrashLoopBackOff
  - ImagePullBackOff
  - Scheduling issues
  - Resource limits exceeded

## UC3 – View Pod Details
- User selects pod → presses Enter.
- Right panel shows:
  - Pod metadata
  - Containers & states
  - Conditions
  - Events

## UC4 – View Logs
- User presses `l` to open logs viewer.
- Scrolls with arrows.
- Switches containers with `c`.

## UC5 – Namespace Health Summary
- User presses `h` on namespace list.
- Summary includes:
  - Pod counts
  - Failed pods
  - Scheduling issues
  - Quota/limit issues

## UC6 – Copy Suggested Commands
- User opens detail/diagnostic view.
- Tool shows:
  ```sh
  kubectl describe pod X -n Y
  kubectl logs X -n Y
