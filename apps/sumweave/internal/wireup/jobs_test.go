package wireup

import (
	"errors"
	"testing"

	"github.com/gemyago/sumweave/apps/sumweave/internal/config"
	"github.com/jaswdr/faker/v2"
	"github.com/stretchr/testify/require"
)

func TestBuildProcessRoots(t *testing.T) {
	fake := faker.New()

	t.Run("scheduler delegates due commands without persistence", func(t *testing.T) {
		bankSchedules := newMockscheduleEnqueuer(t)
		fxSchedules := newMockscheduleEnqueuer(t)
		bankSchedules.EXPECT().EnqueueDue(t.Context()).Return(2, nil).Once()
		fxSchedules.EXPECT().EnqueueDue(t.Context()).Return(3, nil).Once()
		scheduler := &SchedulerRoot{bankSchedules: bankSchedules, fxSchedules: fxSchedules}
		enqueued, err := scheduler.EnqueueDue(t.Context())
		require.NoError(t, err)
		require.Equal(t, 5, enqueued)
	})

	t.Run("scheduler returns the command count before a failed due service", func(t *testing.T) {
		bankSchedules := newMockscheduleEnqueuer(t)
		fxSchedules := newMockscheduleEnqueuer(t)
		expectedErr := errors.New(fake.Lorem().Sentence(3))
		bankSchedules.EXPECT().EnqueueDue(t.Context()).Return(2, nil).Once()
		fxSchedules.EXPECT().EnqueueDue(t.Context()).Return(0, expectedErr).Once()
		scheduler := &SchedulerRoot{bankSchedules: bankSchedules, fxSchedules: fxSchedules}
		enqueued, err := scheduler.EnqueueDue(t.Context())
		require.ErrorIs(t, err, expectedErr)
		require.Equal(t, 2, enqueued)
	})

	t.Run("rejects root settings before opening process resources", func(t *testing.T) {
		_, err := BuildWorker(t.Context(), WorkerOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)
		_, err = BuildScheduler(t.Context(), SchedulerOptions{Environment: fake.UUID().V4()})
		require.Error(t, err)

		t.Setenv("APP_APPLICATION_DATABASE_DSN", "")
		t.Setenv("APP_AGENTRUNTIME_DATABASE_DSN", "")
		values, err := config.LoadValues(config.ValuesLoadInput{Environment: "production"})
		require.NoError(t, err)
		_, err = values.WorkerRoot("production")
		require.ErrorContains(t, err, "application database dsn")
	})
}
