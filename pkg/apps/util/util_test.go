package util

import (
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/internaltest"
	appsinternalutil "github.com/openshift/origin/pkg/apps/controller/util"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestPodName(t *testing.T) {
	deployment := &kapi.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testName",
		},
	}
	expected := "testName-deploy"
	actual := DeployerPodNameForDeployment(deployment.Name)
	if expected != actual {
		t.Errorf("Unexpected pod name for deployment. Expected: %s Got: %s", expected, actual)
	}
}

func TestMakeDeploymentOk(t *testing.T) {
	config := appstest.OkDeploymentConfig(1)
	deployment, err := appsinternalutil.MakeDeploymentV1FromInternalConfig(config)

	if err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}

	expectedAnnotations := map[string]string{
		appsapi.DeploymentConfigAnnotation:  config.Name,
		appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusNew),
		appsapi.DeploymentVersionAnnotation: strconv.FormatInt(config.Status.LatestVersion, 10),
	}

	for key, expected := range expectedAnnotations {
		if actual := deployment.Annotations[key]; actual != expected {
			t.Fatalf("expected deployment annotation %s=%s, got %s", key, expected, actual)
		}
	}

	expectedAnnotations = map[string]string{
		appsapi.DeploymentAnnotation:        deployment.Name,
		appsapi.DeploymentConfigAnnotation:  config.Name,
		appsapi.DeploymentVersionAnnotation: strconv.FormatInt(config.Status.LatestVersion, 10),
	}

	for key, expected := range expectedAnnotations {
		if actual := deployment.Spec.Template.Annotations[key]; actual != expected {
			t.Fatalf("expected pod template annotation %s=%s, got %s", key, expected, actual)
		}
	}

	if len(deployment.Annotations[appsapi.DeploymentEncodedConfigAnnotation]) == 0 {
		t.Fatalf("expected deployment with DeploymentEncodedConfigAnnotation annotation")
	}

	if decodedConfig, err := DecodeDeploymentConfig(deployment); err != nil {
		t.Fatalf("invalid encoded config on deployment: %v", err)
	} else {
		if e, a := config.Name, decodedConfig.Name; e != a {
			t.Fatalf("encoded config name doesn't match source config")
		}
		// TODO: more assertions
	}

	if *deployment.Spec.Replicas != 0 {
		t.Fatalf("expected deployment replicas to be 0 but has %d", *deployment.Spec.Replicas)
	}

	if l, e, a := appsapi.DeploymentConfigAnnotation, config.Name, deployment.Labels[appsapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("expected label %s=%s, got %s", l, e, a)
	}

	if e, a := config.Name, deployment.Spec.Template.Labels[appsapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected label DeploymentConfigLabel=%s, got %s", e, a)
	}

	if e, a := deployment.Name, deployment.Spec.Template.Labels[appsapi.DeploymentLabel]; e != a {
		t.Fatalf("expected label DeploymentLabel=%s, got %s", e, a)
	}

	if e, a := config.Name, deployment.Spec.Selector[appsapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected selector DeploymentConfigLabel=%s, got %s", e, a)
	}

	if e, a := deployment.Name, deployment.Spec.Selector[appsapi.DeploymentLabel]; e != a {
		t.Fatalf("expected selector DeploymentLabel=%s, got %s", e, a)
	}
}

func TestDeploymentsByLatestVersion_sorting(t *testing.T) {
	mkdeployment := func(version int64) *v1.ReplicationController {
		deployment, _ := appsinternalutil.MakeDeploymentV1FromInternalConfig(appstest.OkDeploymentConfig(version))
		return deployment
	}
	deployments := []*v1.ReplicationController{
		mkdeployment(4),
		mkdeployment(1),
		mkdeployment(2),
		mkdeployment(3),
	}
	sort.Sort(ByLatestVersionAsc(deployments))
	for i := int64(0); i < 4; i++ {
		if e, a := i+1, DeploymentVersionFor(deployments[i]); e != a {
			t.Errorf("expected deployment[%d]=%d, got %d", i, e, a)
		}
	}
	sort.Sort(ByLatestVersionDesc(deployments))
	for i := int64(0); i < 4; i++ {
		if e, a := 4-i, DeploymentVersionFor(deployments[i]); e != a {
			t.Errorf("expected deployment[%d]=%d, got %d", i, e, a)
		}
	}
}

func TestCanTransitionPhase(t *testing.T) {
	tests := []struct {
		name          string
		current, next appsapi.DeploymentStatus
		expected      bool
	}{
		{
			name:     "New->New",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "New->Pending",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusPending,
			expected: true,
		},
		{
			name:     "New->Running",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusRunning,
			expected: true,
		},
		{
			name:     "New->Complete",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusComplete,
			expected: true,
		},
		{
			name:     "New->Failed",
			current:  appsapi.DeploymentStatusNew,
			next:     appsapi.DeploymentStatusFailed,
			expected: true,
		},
		{
			name:     "Pending->New",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Pending->Pending",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Pending->Running",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusRunning,
			expected: true,
		},
		{
			name:     "Pending->Failed",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusFailed,
			expected: true,
		},
		{
			name:     "Pending->Complete",
			current:  appsapi.DeploymentStatusPending,
			next:     appsapi.DeploymentStatusComplete,
			expected: true,
		},
		{
			name:     "Running->New",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Running->Pending",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Running->Running",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusRunning,
			expected: false,
		},
		{
			name:     "Running->Failed",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusFailed,
			expected: true,
		},
		{
			name:     "Running->Complete",
			current:  appsapi.DeploymentStatusRunning,
			next:     appsapi.DeploymentStatusComplete,
			expected: true,
		},
		{
			name:     "Complete->New",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Complete->Pending",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Complete->Running",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusRunning,
			expected: false,
		},
		{
			name:     "Complete->Failed",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusFailed,
			expected: false,
		},
		{
			name:     "Complete->Complete",
			current:  appsapi.DeploymentStatusComplete,
			next:     appsapi.DeploymentStatusComplete,
			expected: false,
		},
		{
			name:     "Failed->New",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusNew,
			expected: false,
		},
		{
			name:     "Failed->Pending",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusPending,
			expected: false,
		},
		{
			name:     "Failed->Running",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusRunning,
			expected: false,
		},
		{
			name:     "Failed->Complete",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusComplete,
			expected: false,
		},
		{
			name:     "Failed->Failed",
			current:  appsapi.DeploymentStatusFailed,
			next:     appsapi.DeploymentStatusFailed,
			expected: false,
		},
	}

	for _, test := range tests {
		got := appsinternalutil.CanTransitionPhase(test.current, test.next)
		if got != test.expected {
			t.Errorf("%s: expected %t, got %t", test.name, test.expected, got)
		}
	}
}

var (
	now     = metav1.Now()
	later   = metav1.Time{Time: now.Add(time.Minute)}
	earlier = metav1.Time{Time: now.Add(-time.Minute)}

	condProgressing = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:               appsapi.DeploymentProgressing,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: now,
		}
	}

	condProgressingDifferentTime = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:               appsapi.DeploymentProgressing,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: later,
		}
	}

	condProgressingDifferentReason = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:               appsapi.DeploymentProgressing,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: later,
			Reason:             appsapi.NewReplicationControllerReason,
		}
	}

	condNotProgressing = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:               appsapi.DeploymentProgressing,
			Status:             kapi.ConditionFalse,
			LastUpdateTime:     earlier,
			LastTransitionTime: earlier,
		}
	}

	condAvailable = func() appsapi.DeploymentCondition {
		return appsapi.DeploymentCondition{
			Type:   appsapi.DeploymentAvailable,
			Status: kapi.ConditionTrue,
		}
	}
)

func TestGetCondition(t *testing.T) {
	exampleStatus := func() appsapi.DeploymentConfigStatus {
		return appsapi.DeploymentConfigStatus{
			Conditions: []appsapi.DeploymentCondition{condProgressing(), condAvailable()},
		}
	}

	tests := []struct {
		name string

		status     appsapi.DeploymentConfigStatus
		condType   appsapi.DeploymentConditionType
		condStatus kapi.ConditionStatus

		expected bool
	}{
		{
			name: "condition exists",

			status:   exampleStatus(),
			condType: appsapi.DeploymentAvailable,

			expected: true,
		},
		{
			name: "condition does not exist",

			status:   exampleStatus(),
			condType: appsapi.DeploymentReplicaFailure,

			expected: false,
		},
	}

	for _, test := range tests {
		cond := appsinternalutil.GetDeploymentCondition(test.status, test.condType)
		exists := cond != nil
		if exists != test.expected {
			t.Errorf("%s: expected condition to exist: %t, got: %t", test.name, test.expected, exists)
		}
	}
}

func TestSetCondition(t *testing.T) {
	tests := []struct {
		name string

		status *appsapi.DeploymentConfigStatus
		cond   appsapi.DeploymentCondition

		expectedStatus *appsapi.DeploymentConfigStatus
	}{
		{
			name: "set for the first time",

			status: &appsapi.DeploymentConfigStatus{},
			cond:   condAvailable(),

			expectedStatus: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condAvailable(),
				},
			},
		},
		{
			name: "simple set",

			status: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condProgressing(),
				},
			},
			cond: condAvailable(),

			expectedStatus: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condProgressing(), condAvailable(),
				},
			},
		},
		{
			name: "replace if status changes",

			status: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condNotProgressing(),
				},
			},
			cond: condProgressing(),

			expectedStatus: &appsapi.DeploymentConfigStatus{Conditions: []appsapi.DeploymentCondition{condProgressing()}},
		},
		{
			name: "replace if reason changes",

			status: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condProgressing(),
				},
			},
			cond: condProgressingDifferentReason(),

			expectedStatus: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					{
						Type:   appsapi.DeploymentProgressing,
						Status: kapi.ConditionTrue,
						// Note that LastTransitionTime stays the same.
						LastTransitionTime: now,
						// Only the reason changes.
						Reason: appsapi.NewReplicationControllerReason,
					},
				},
			},
		},
		{
			name: "don't replace if status and reason don't change",

			status: &appsapi.DeploymentConfigStatus{
				Conditions: []appsapi.DeploymentCondition{
					condProgressing(),
				},
			},
			cond: condProgressingDifferentTime(),

			expectedStatus: &appsapi.DeploymentConfigStatus{Conditions: []appsapi.DeploymentCondition{condProgressing()}},
		},
	}

	for _, test := range tests {
		t.Logf("running test %q", test.name)
		appsinternalutil.SetDeploymentCondition(test.status, test.cond)
		if !reflect.DeepEqual(test.status, test.expectedStatus) {
			t.Errorf("expected status: %v, got: %v", test.expectedStatus, test.status)
		}
	}
}

func TestRemoveCondition(t *testing.T) {
	exampleStatus := func() *appsapi.DeploymentConfigStatus {
		return &appsapi.DeploymentConfigStatus{
			Conditions: []appsapi.DeploymentCondition{condProgressing(), condAvailable()},
		}
	}

	tests := []struct {
		name string

		status   *appsapi.DeploymentConfigStatus
		condType appsapi.DeploymentConditionType

		expectedStatus *appsapi.DeploymentConfigStatus
	}{
		{
			name: "remove from empty status",

			status:   &appsapi.DeploymentConfigStatus{},
			condType: appsapi.DeploymentProgressing,

			expectedStatus: &appsapi.DeploymentConfigStatus{},
		},
		{
			name: "simple remove",

			status:   &appsapi.DeploymentConfigStatus{Conditions: []appsapi.DeploymentCondition{condProgressing()}},
			condType: appsapi.DeploymentProgressing,

			expectedStatus: &appsapi.DeploymentConfigStatus{},
		},
		{
			name: "doesn't remove anything",

			status:   exampleStatus(),
			condType: appsapi.DeploymentReplicaFailure,

			expectedStatus: exampleStatus(),
		},
	}

	for _, test := range tests {
		appsinternalutil.RemoveDeploymentCondition(test.status, test.condType)
		if !reflect.DeepEqual(test.status, test.expectedStatus) {
			t.Errorf("%s: expected status: %v, got: %v", test.name, test.expectedStatus, test.status)
		}
	}
}

func TestRolloutExceededTimeoutSeconds(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                   string
		config                 *appsapi.DeploymentConfig
		deploymentCreationTime time.Time
		expectTimeout          bool
	}{
		// Recreate strategy with deployment running for 20s (exceeding 10s timeout)
		{
			name: "recreate timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy.RecreateParams.TimeoutSeconds = &timeoutSeconds
				return config
			}(int64(10)),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          true,
		},
		// Recreate strategy with no timeout
		{
			name: "recreate no timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy.RecreateParams.TimeoutSeconds = &timeoutSeconds
				return config
			}(int64(0)),
			deploymentCreationTime: now.Add(-700 * time.Second),
			expectTimeout:          false,
		},

		// Rolling strategy with deployment running for 20s (exceeding 10s timeout)
		{
			name: "rolling timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy = appstest.OkRollingStrategy()
				config.Spec.Strategy.RollingParams.TimeoutSeconds = &timeoutSeconds
				return config
			}(int64(10)),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          true,
		},
		// Rolling strategy with deployment with no timeout specified.
		{
			name: "rolling using default timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy = appstest.OkRollingStrategy()
				config.Spec.Strategy.RollingParams.TimeoutSeconds = nil
				return config
			}(0),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          false,
		},
		// Recreate strategy with deployment with no timeout specified.
		{
			name: "recreate using default timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy.RecreateParams.TimeoutSeconds = nil
				return config
			}(0),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          false,
		},
		// Custom strategy with deployment with no timeout specified.
		{
			name: "custom using default timeout",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy = appstest.OkCustomStrategy()
				return config
			}(0),
			deploymentCreationTime: now.Add(-20 * time.Second),
			expectTimeout:          false,
		},
		// Custom strategy use default timeout exceeding it.
		{
			name: "custom using default timeout timing out",
			config: func(timeoutSeconds int64) *appsapi.DeploymentConfig {
				config := appstest.OkDeploymentConfig(1)
				config.Spec.Strategy = appstest.OkCustomStrategy()
				return config
			}(0),
			deploymentCreationTime: now.Add(-700 * time.Second),
			expectTimeout:          true,
		},
	}

	for _, tc := range tests {
		config := tc.config
		deployment, err := appsinternalutil.MakeDeploymentV1FromInternalConfig(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		deployment.ObjectMeta.CreationTimestamp = metav1.Time{Time: tc.deploymentCreationTime}
		gotTimeout := appsinternalutil.RolloutExceededTimeoutSeconds(config, deployment)
		if tc.expectTimeout && !gotTimeout {
			t.Errorf("[%s]: expected timeout, but got no timeout", tc.name)
		}
		if !tc.expectTimeout && gotTimeout {
			t.Errorf("[%s]: expected no timeout, but got timeout", tc.name)
		}

	}
}
