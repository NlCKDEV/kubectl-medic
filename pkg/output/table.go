package output

import (
	"fmt"
	"text/tabwriter"

	"os"

	"github.com/NlCKDEV/kubectl-medic/pkg/scanner"
)

// PrintIssuesTable renders issues in a simple table.
func PrintIssuesTable(issues []scanner.Issue) {
	if len(issues) == 0 {
		fmt.Println("No issues found 🎉")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tPOD\tPHASE\tSEVERITY\tSUGGESTION")

	for _, iss := range issues {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			iss.Namespace, iss.PodName, iss.Phase, iss.Severity, iss.Suggestion)
	}

	w.Flush()
}
