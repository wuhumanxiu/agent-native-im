DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'agent_im') THEN
    REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLE file_records FROM agent_im;
    REVOKE USAGE, SELECT ON SEQUENCE file_records_id_seq FROM agent_im;
  END IF;
END $$;
