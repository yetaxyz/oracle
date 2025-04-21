import React from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  CircularProgress,
  Box,
  Chip,
  Tooltip
} from '@mui/material';
import { formatDistanceToNow } from 'date-fns';

// Helper function for number formatting (can be moved to a utils file)
const formatNumber = (num, options = {}) => {
  const defaultOptions = { maximumFractionDigits: 2 };
  return new Intl.NumberFormat('en-US', { ...defaultOptions, ...options }).format(num);
};

const formatCurrency = (num) => {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(num);
};

const getStatusChip = (status) => {
  let color = 'default';
  let label = status ? status.toUpperCase() : 'UNKNOWN';

  switch (status) {
    case 'valid':
      color = 'success';
      break;
    case 'outlier':
      color = 'warning';
      break;
    case 'stale':
      color = 'error';
      break;
    default:
      break;
  }
  return <Chip label={label} color={color} size="small" sx={{ fontWeight: 'medium' }} />;
};

const SourceDetailModal = ({ open, onClose, symbol, details, loading, error }) => {
  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth scroll="paper">
      <DialogTitle>
        Source Details: <Typography component="span" variant="h6" color="primary">{symbol}</Typography>
      </DialogTitle>
      <DialogContent dividers>
        {loading && (
          <Box display="flex" justifyContent="center" my={3}>
            <CircularProgress />
          </Box>
        )}
        {error && (
          <Typography color="error" align="center" my={3}>
            Error fetching source details: {error}
          </Typography>
        )}
        {!loading && !error && details && (
          <TableContainer>
            <Table stickyHeader size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Source</TableCell>
                  <TableCell align="right">Raw Price</TableCell>
                  <TableCell align="right">Volume</TableCell>
                  <TableCell>Timestamp</TableCell>
                  <TableCell align="center">Status</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {(details || []).map((row, index) => (
                  <TableRow key={`${row.source}-${index}`} hover>
                    <TableCell>{row.source}</TableCell>
                    <TableCell align="right">{formatCurrency(row.price)}</TableCell>
                    <TableCell align="right">{formatNumber(row.volume)}</TableCell>
                    <TableCell>
                       <Tooltip title={new Date(row.timestamp).toLocaleString()} placement="top">
                         <span>{formatDistanceToNow(new Date(row.timestamp), { addSuffix: true })}</span>
                       </Tooltip>
                    </TableCell>
                    <TableCell align="center">{getStatusChip(row.status)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
         {!loading && !error && (!details || details.length === 0) && (
             <Typography align="center" my={3}>No source details available for this symbol.</Typography>
         )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
};

export default SourceDetailModal; 