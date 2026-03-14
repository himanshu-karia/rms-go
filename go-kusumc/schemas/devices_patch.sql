-- Idempotent patch to ensure device metadata columns exist
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'devices'
		  AND column_name = 'name'
	) THEN
		ALTER TABLE devices ADD COLUMN name TEXT;
	END IF;

	IF NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'devices'
		  AND column_name = 'status'
	) THEN
		ALTER TABLE devices ADD COLUMN status TEXT;
	END IF;
END $$;
