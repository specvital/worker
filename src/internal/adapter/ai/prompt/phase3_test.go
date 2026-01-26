package prompt

import (
	"strings"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestBuildPhase3UserPrompt(t *testing.T) {
	input := specview.Phase3Input{
		Language: "Korean",
		Domains: []specview.Domain{
			{
				Name:        "인증",
				Description: "사용자 인증 관련 테스트",
				Features: []specview.Feature{
					{
						Name:        "로그인",
						Description: "로그인 기능 검증",
						Behaviors: []specview.Behavior{
							{Description: "유효한 자격 증명으로 로그인 성공"},
							{Description: "잘못된 비밀번호로 로그인 실패"},
						},
					},
				},
			},
			{
				Name:        "결제",
				Description: "결제 처리 관련 테스트",
				Features: []specview.Feature{
					{
						Name:        "카드 결제",
						Description: "카드 결제 흐름 검증",
						Behaviors: []specview.Behavior{
							{Description: "유효한 카드로 결제 성공"},
						},
					},
				},
			},
		},
	}

	result := BuildPhase3UserPrompt(input)

	if !strings.Contains(result, "Target Language: Korean") {
		t.Error("should contain target language")
	}
	if !strings.Contains(result, "<document_structure>") {
		t.Error("should contain document_structure tag")
	}
	if !strings.Contains(result, "## 인증") {
		t.Error("should contain domain name")
	}
	if !strings.Contains(result, "### 로그인") {
		t.Error("should contain feature name")
	}
	if !strings.Contains(result, "- 유효한 자격 증명으로 로그인 성공") {
		t.Error("should contain behavior description")
	}
	if !strings.Contains(result, "2 domains, 3 total behaviors") {
		t.Error("should contain summary counts")
	}
}

func TestBuildPhase3UserPrompt_emptyDomains(t *testing.T) {
	input := specview.Phase3Input{
		Language: "English",
		Domains:  []specview.Domain{},
	}

	result := BuildPhase3UserPrompt(input)

	if !strings.Contains(result, "0 domains, 0 total behaviors") {
		t.Error("should handle empty domains with zero counts")
	}
}
