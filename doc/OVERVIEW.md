# kubectl-medic – Overview

`kubectl-medic` is a kubectl plugin that provides a guided, simple, visual way to troubleshoot Kubernetes pods and namespaces using a TUI.

The aim is to reduce cognitive load for users unfamiliar with Kubernetes while providing value to experts through fast triage capabilities.

---

## Key Objectives

1. Provide a minimal but powerful visual troubleshooting tool.
2. Reduce reliance on memorizing kubectl commands.
3. Highlight issues that are not obvious from raw Kubernetes output.
4. Maintain safety: primarily read-only with optional restart action.
5. Provide consistent styling, predictable keybindings, and simple navigation.

---

## Main Capabilities (v1)

- Namespace browsing
- Pod listing with health indicators
- Pod detail + events view
- Logs viewer
- Diagnostics engine
- Namespace health summary
- Suggested kubectl commands for manual execution

---

## Philosophy

> **A doctor who diagnoses, not a surgeon who performs operations.**

The tool should tell users **what might be wrong** and **which command to run**, but never automatically fix things.

---

## Target Users

- Junior developers
- On-call engineers
- Platform engineers wanting a compact troubleshooting dashboard

