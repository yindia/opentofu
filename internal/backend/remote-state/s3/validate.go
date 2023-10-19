package s3

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/opentofu/opentofu/internal/tfdiags"
	"github.com/zclconf/go-cty/cty"
)

const (
	multiRegionKeyIdPattern = `mrk-[a-f0-9]{32}`
	uuidRegexPattern        = `[a-f0-9]{8}-[a-f0-9]{4}-[1-5][a-f0-9]{3}-[ab89][a-f0-9]{3}-[a-f0-9]{12}`
	aliasRegexPattern       = `alias/[a-zA-Z0-9/_-]+`
)

func validateKMSKey(path cty.Path, s string) (diags tfdiags.Diagnostics) {
	if arn.IsARN(s) {
		return validateKMSKeyARN(path, s)
	}
	return validateKMSKeyID(path, s)
}

func validateKMSKeyID(path cty.Path, s string) (diags tfdiags.Diagnostics) {
	keyIdRegex := regexp.MustCompile(`^` + uuidRegexPattern + `|` + multiRegionKeyIdPattern + `|` + aliasRegexPattern + `$`)
	if !keyIdRegex.MatchString(s) {
		diags = diags.Append(tfdiags.AttributeValue(
			tfdiags.Error,
			"Invalid KMS Key ID",
			fmt.Sprintf("Value must be a valid KMS Key ID, got %q", s),
			path,
		))
		return diags
	}

	return diags
}

func validateKMSKeyARN(path cty.Path, s string) (diags tfdiags.Diagnostics) {
	parsedARN, err := arn.Parse(s)
	if err != nil {
		diags = diags.Append(tfdiags.AttributeValue(
			tfdiags.Error,
			"Invalid KMS Key ARN",
			fmt.Sprintf("Value must be a valid KMS Key ARN, got %q", s),
			path,
		))
		return diags
	}

	if !isKeyARN(parsedARN) {
		diags = diags.Append(tfdiags.AttributeValue(
			tfdiags.Error,
			"Invalid KMS Key ARN",
			fmt.Sprintf("Value must be a valid KMS Key ARN, got %q", s),
			path,
		))
		return diags
	}

	return diags
}

func isKeyARN(arn arn.ARN) bool {
	return keyIdFromARNResource(arn.Resource) != "" || aliasIdFromARNResource(arn.Resource) != ""
}

func keyIdFromARNResource(s string) string {
	keyIdResourceRegex := regexp.MustCompile(`^key/(` + uuidRegexPattern + `|` + multiRegionKeyIdPattern + `)$`)
	matches := keyIdResourceRegex.FindStringSubmatch(s)
	if matches == nil || len(matches) != 2 {
		return ""
	}

	return matches[1]
}

func aliasIdFromARNResource(s string) string {
	aliasIdResourceRegex := regexp.MustCompile(`^(` + aliasRegexPattern + `)$`)
	matches := aliasIdResourceRegex.FindStringSubmatch(s)
	if matches == nil || len(matches) != 2 {
		return ""
	}

	return matches[1]
}

type objectValidator func(obj cty.Value, objPath cty.Path, diags *tfdiags.Diagnostics)

func validateAttributesConflict(paths ...cty.Path) objectValidator {
	return func(obj cty.Value, objPath cty.Path, diags *tfdiags.Diagnostics) {
		found := false
		for _, path := range paths {
			val, err := path.Apply(obj)
			if err != nil {
				*diags = diags.Append(attributeErrDiag(
					"Invalid Path for Schema",
					"The S3 Backend unexpectedly provided a path that does not match the schema. "+
						"Please report this to the developers.\n\n"+
						"Path: "+pathString(path)+"\n\n"+
						"Error: "+err.Error(),
					objPath,
				))
				continue
			}
			if !val.IsNull() {
				if found {
					pathStrs := make([]string, len(paths))
					for i, path := range paths {
						pathStrs[i] = pathString(path)
					}
					*diags = diags.Append(attributeErrDiag(
						"Invalid Attribute Combination",
						fmt.Sprintf(`Only one of %s can be set.`, strings.Join(pathStrs, ", ")),
						objPath,
					))
					return
				}
				found = true
			}
		}
	}
}

func attributeErrDiag(summary, detail string, attrPath cty.Path) tfdiags.Diagnostic {
	return tfdiags.AttributeValue(tfdiags.Error, summary, detail, attrPath.Copy())
}

func attributeWarningDiag(summary, detail string, attrPath cty.Path) tfdiags.Diagnostic {
	return tfdiags.AttributeValue(tfdiags.Warning, summary, detail, attrPath.Copy())
}
